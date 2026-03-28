<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import UserSearchSelect from './components/UserSearchSelect.vue'
import { apiRequest, buildWsUrl, formatTime, renderMessageHtml, shortId } from './lib/api'

const SESSION_KEY = 'mygo.session'

const storedSession = parseStoredSession()

const booting = ref(Boolean(storedSession?.token))
const authMode = ref('login')
const authBusy = ref(false)
const authError = ref('')
const workspaceError = ref('')
const sessionToken = ref(storedSession?.token || '')
const currentUser = ref(storedSession?.user || null)
const conversations = ref([])
const activeConversationId = ref('')
const messages = ref([])
const members = ref([])
const composer = ref('')
const conversationBusy = ref(false)
const memberBusy = ref(false)
const sendBusy = ref(false)
const uploadBusy = ref(false)
const createBusy = ref(false)
const inviteBusy = ref(false)
const createModalOpen = ref(false)
const wsState = ref('未连接')
const ws = ref(null)
const messageScroller = ref(null)

const authForm = reactive({
  email: '',
  username: '',
  displayName: '',
  password: '',
  identity: ''
})

const createForm = reactive({
  name: '',
  selectedUsers: []
})

const inviteUsers = ref([])
let reconnectTimer = null
let statusTimer = null
let manualClose = false
let knownMessageIds = new Set()
let subscribedConversationId = ''

const activeConversation = computed(() =>
  conversations.value.find((conversation) => conversation.id === activeConversationId.value) || null
)

const memberDirectory = computed(() => {
  const directory = new Map()
  for (const member of members.value) {
    directory.set(
      member.user_id,
      member.display_name || member.username || member.email || shortId(member.user_id)
    )
  }
  return directory
})

const canManageMembers = computed(() =>
  ['owner', 'admin'].includes(activeConversation.value?.member_role || '')
)

const isReady = computed(() => Boolean(sessionToken.value && currentUser.value))

watch(
  () => messages.value.length,
  async () => {
    await nextTick()
    if (messageScroller.value) {
      messageScroller.value.scrollTop = messageScroller.value.scrollHeight
    }
  }
)

onMounted(async () => {
  if (sessionToken.value) {
    await restoreSession()
  } else {
    booting.value = false
  }
})

onBeforeUnmount(() => {
  disconnectSocket()
})

function parseStoredSession() {
  try {
    const raw = localStorage.getItem(SESSION_KEY)
    return raw ? JSON.parse(raw) : null
  } catch {
    return null
  }
}

function saveSession(token, user) {
  sessionToken.value = token
  currentUser.value = user
  localStorage.setItem(SESSION_KEY, JSON.stringify({ token, user }))
}

function clearSession() {
  disconnectSocket()
  sessionToken.value = ''
  currentUser.value = null
  conversations.value = []
  activeConversationId.value = ''
  messages.value = []
  members.value = []
  composer.value = ''
  createForm.name = ''
  createForm.selectedUsers = []
  inviteUsers.value = []
  localStorage.removeItem(SESSION_KEY)
}

async function restoreSession() {
  try {
    workspaceError.value = ''
    currentUser.value = await apiRequest('/me', { token: sessionToken.value })
    await loadConversations(false)
    connectSocket()
  } catch (error) {
    clearSession()
    authError.value = '登录状态已失效，请重新登录'
  } finally {
    booting.value = false
  }
}

async function submitAuth() {
  authBusy.value = true
  authError.value = ''

  try {
    const path = authMode.value === 'register' ? '/auth/register' : '/auth/login'
    const payload =
      authMode.value === 'register'
        ? {
            email: authForm.email,
            username: authForm.username,
            display_name: authForm.displayName,
            password: authForm.password
          }
        : {
            email_or_username: authForm.identity,
            password: authForm.password
          }

    const data = await apiRequest(path, {
      method: 'POST',
      body: payload
    })

    saveSession(data.token.access_token, data.user)
    resetAuthForm()
    await loadConversations(false)
    connectSocket()
  } catch (error) {
    authError.value = error.message
  } finally {
    authBusy.value = false
    booting.value = false
  }
}

function resetAuthForm() {
  authForm.email = ''
  authForm.username = ''
  authForm.displayName = ''
  authForm.password = ''
  authForm.identity = ''
}

async function loadConversations(keepSelection = true) {
  if (!sessionToken.value) {
    return
  }

  const data = await apiRequest('/conversations', {
    token: sessionToken.value
  })

  conversations.value = data

  if (!data.length) {
    activeConversationId.value = ''
    messages.value = []
    members.value = []
    knownMessageIds = new Set()
    return
  }

  const stillExists = data.some((conversation) => conversation.id === activeConversationId.value)
  const nextConversationId =
    keepSelection && stillExists ? activeConversationId.value : data[0].id

  if (nextConversationId && nextConversationId !== activeConversationId.value) {
    await openConversation(nextConversationId)
  }
}

async function openConversation(conversationId) {
  if (!conversationId || conversationId === activeConversationId.value && conversationBusy.value) {
    return
  }

  const previousConversationId = activeConversationId.value
  activeConversationId.value = conversationId
  workspaceError.value = ''
  conversationBusy.value = true
  memberBusy.value = true

  if (previousConversationId && previousConversationId !== conversationId) {
    sendSocketMessage({
      type: 'unsubscribe',
      conversation_id: previousConversationId
    })
  }

  try {
    const [messageData, memberData] = await Promise.all([
      apiRequest(`/conversations/${conversationId}/messages`, {
        token: sessionToken.value
      }),
      apiRequest(`/conversations/${conversationId}/members`, {
        token: sessionToken.value
      })
    ])

    messages.value = messageData
    members.value = memberData
    knownMessageIds = new Set(messageData.map((message) => message.id))

    subscribeConversation(conversationId)
    await markConversationRead()
  } catch (error) {
    workspaceError.value = error.message
  } finally {
    conversationBusy.value = false
    memberBusy.value = false
  }
}

function connectSocket() {
  if (!sessionToken.value) {
    return
  }

  disconnectSocket()
  manualClose = false
  wsState.value = '连接中'

  const socket = new WebSocket(buildWsUrl(sessionToken.value))
  ws.value = socket

  socket.onopen = () => {
    wsState.value = '实时已连接'
    if (activeConversationId.value) {
      subscribeConversation(activeConversationId.value)
    }
  }

  socket.onmessage = async (event) => {
    const envelope = JSON.parse(event.data)

    if (envelope.type === 'chat.message.created') {
      const { conversation_id: conversationId, message } = envelope.data

      if (conversationId === activeConversationId.value && !knownMessageIds.has(message.id)) {
        messages.value = [...messages.value, message]
        knownMessageIds.add(message.id)

        if (message.sender_id !== currentUser.value?.id) {
          await markConversationRead(message.id, false)
        }
      }

      await loadConversations(true)
      return
    }

    if (envelope.type === 'system.error') {
      workspaceError.value = envelope.data?.message || '实时连接发生错误'
    }
  }

  socket.onerror = () => {
    wsState.value = '连接异常'
  }

  socket.onclose = () => {
    ws.value = null
    subscribedConversationId = ''
    wsState.value = '已断开'

    if (!manualClose && sessionToken.value) {
      clearTimeout(reconnectTimer)
      reconnectTimer = setTimeout(() => {
        connectSocket()
      }, 1800)
    }
  }
}

function disconnectSocket() {
  manualClose = true
  clearTimeout(reconnectTimer)
  clearTimeout(statusTimer)

  if (ws.value) {
    ws.value.close()
    ws.value = null
  }
  subscribedConversationId = ''
}

function subscribeConversation(conversationId) {
  if (!conversationId || !ws.value || ws.value.readyState !== WebSocket.OPEN) {
    return
  }

  if (subscribedConversationId === conversationId) {
    return
  }

  sendSocketMessage({
    type: 'subscribe',
    conversation_id: conversationId
  })
  subscribedConversationId = conversationId
}

function sendSocketMessage(payload) {
  if (!ws.value || ws.value.readyState !== WebSocket.OPEN) {
    return
  }

  ws.value.send(JSON.stringify(payload))
}

async function sendMessage(customContent = '') {
  if (!activeConversationId.value) {
    return
  }

  const content = (customContent || composer.value).trim()
  if (!content) {
    return
  }

  sendBusy.value = true
  workspaceError.value = ''

  try {
    const message = await apiRequest(`/conversations/${activeConversationId.value}/messages`, {
      method: 'POST',
      token: sessionToken.value,
      body: {
        content
      }
    })

    composer.value = ''
    if (!knownMessageIds.has(message.id)) {
      messages.value = [...messages.value, message]
      knownMessageIds.add(message.id)
    }

    await markConversationRead(message.id, false)
    await loadConversations(true)
  } catch (error) {
    workspaceError.value = error.message
  } finally {
    sendBusy.value = false
  }
}

async function handleAttachmentUpload(event) {
  const file = event.target.files?.[0]
  event.target.value = ''

  if (!file || !activeConversationId.value) {
    return
  }

  uploadBusy.value = true
  workspaceError.value = ''

  try {
    const formData = new FormData()
    formData.append('file', file)

    const uploaded = await apiRequest(`/conversations/${activeConversationId.value}/attachments`, {
      method: 'POST',
      token: sessionToken.value,
      body: formData
    })

    await sendMessage(`上传了附件：${uploaded.original_name}\n${uploaded.public_url}`)
  } catch (error) {
    workspaceError.value = error.message
  } finally {
    uploadBusy.value = false
  }
}

async function markConversationRead(lastMessageId = '', refreshList = true) {
  if (!activeConversationId.value || !messages.value.length) {
    return
  }

  const lastId = lastMessageId || messages.value[messages.value.length - 1]?.id
  if (!lastId) {
    return
  }

  try {
    await apiRequest(`/conversations/${activeConversationId.value}/read`, {
      method: 'POST',
      token: sessionToken.value,
      body: {
        last_read_message_id: lastId
      }
    })

    conversations.value = conversations.value.map((conversation) =>
      conversation.id === activeConversationId.value
        ? { ...conversation, unread_count: 0 }
        : conversation
    )

    if (refreshList) {
      await loadConversations(true)
    }
  } catch (error) {
    workspaceError.value = error.message
  }
}

async function createConversation() {
  if (!createForm.name.trim()) {
    workspaceError.value = '会话名称不能为空'
    return
  }

  createBusy.value = true
  workspaceError.value = ''

  try {
    const created = await apiRequest('/conversations', {
      method: 'POST',
      token: sessionToken.value,
      body: {
        name: createForm.name.trim(),
        kind: 'group',
        member_ids: createForm.selectedUsers.map((user) => user.id)
      }
    })

    createModalOpen.value = false
    createForm.name = ''
    createForm.selectedUsers = []
    await loadConversations(false)
    await openConversation(created.id)
  } catch (error) {
    workspaceError.value = error.message
  } finally {
    createBusy.value = false
  }
}

async function addMembers() {
  if (!activeConversationId.value || !inviteUsers.value.length) {
    return
  }

  inviteBusy.value = true
  workspaceError.value = ''

  try {
    await apiRequest(`/conversations/${activeConversationId.value}/members`, {
      method: 'POST',
      token: sessionToken.value,
      body: {
        members: inviteUsers.value.map((user) => ({
          user_id: user.id,
          role: 'member'
        }))
      }
    })

    inviteUsers.value = []
    members.value = await apiRequest(`/conversations/${activeConversationId.value}/members`, {
      token: sessionToken.value
    })
    await loadConversations(true)
  } catch (error) {
    workspaceError.value = error.message
  } finally {
    inviteBusy.value = false
  }
}

function logout() {
  clearSession()
  authMode.value = 'login'
}

async function copyUserId() {
  if (!currentUser.value?.id) {
    return
  }

  try {
    await navigator.clipboard.writeText(currentUser.value.id)
    workspaceError.value = ''
    wsState.value = '用户 ID 已复制'
    clearTimeout(statusTimer)
    statusTimer = setTimeout(() => {
      wsState.value = ws.value ? '实时已连接' : '未连接'
    }, 1400)
  } catch {
    workspaceError.value = '浏览器不允许复制用户 ID'
  }
}

function senderLabel(message) {
  if (message.sender_id === currentUser.value?.id) {
    return '我'
  }
  return memberDirectory.value.get(message.sender_id) || shortId(message.sender_id)
}
</script>

<template>
  <div class="app-shell">
    <section v-if="booting" class="boot-screen">
      <div class="boot-card">
        <p class="eyebrow">mygo</p>
        <h1>正在恢复你的会话</h1>
        <p>前端正在连接后端并同步你的聊天数据。</p>
      </div>
    </section>

    <section v-else-if="!isReady" class="auth-screen">
      <div class="auth-hero">
        <p class="eyebrow">mygo</p>
        <h1>实时聊天，不等前端完备再开始。</h1>
        <p class="hero-copy">
          这一版前端已经能直接登录、建会话、实时聊天、拉成员、上传附件和查看未读变化。
        </p>

        <div class="hero-grid">
          <article class="hero-card">
            <strong>JWT 登录</strong>
            <span>注册或登录后，前端会自动保留会话。</span>
          </article>
          <article class="hero-card">
            <strong>实时消息</strong>
            <span>当前会话会自动建立 WebSocket 订阅。</span>
          </article>
          <article class="hero-card">
            <strong>附件上传</strong>
            <span>上传成功后会直接把文件链接发进会话。</span>
          </article>
        </div>
      </div>

      <div class="auth-card">
        <div class="auth-tabs">
          <button
            type="button"
            class="tab-btn"
            :class="{ 'tab-btn--active': authMode === 'login' }"
            @click="authMode = 'login'"
          >
            登录
          </button>
          <button
            type="button"
            class="tab-btn"
            :class="{ 'tab-btn--active': authMode === 'register' }"
            @click="authMode = 'register'"
          >
            注册
          </button>
        </div>

        <form class="auth-form" @submit.prevent="submitAuth">
          <template v-if="authMode === 'register'">
            <label class="field">
              <span>邮箱</span>
              <input v-model="authForm.email" type="email" placeholder="you@example.com" required />
            </label>

            <label class="field">
              <span>用户名</span>
              <input v-model="authForm.username" type="text" placeholder="mygo_user" required />
            </label>

            <label class="field">
              <span>显示名</span>
              <input
                v-model="authForm.displayName"
                type="text"
                placeholder="团队里的名字"
                required
              />
            </label>
          </template>

          <template v-else>
            <label class="field">
              <span>邮箱或用户名</span>
              <input
                v-model="authForm.identity"
                type="text"
                placeholder="邮箱或用户名"
                required
              />
            </label>
          </template>

          <label class="field">
            <span>密码</span>
            <input
              v-model="authForm.password"
              type="password"
              placeholder="至少 6 位"
              required
            />
          </label>

          <p v-if="authError" class="inline-error">{{ authError }}</p>

          <button type="submit" class="primary-btn" :disabled="authBusy">
            {{ authBusy ? '提交中…' : authMode === 'login' ? '进入 mygo' : '创建账户' }}
          </button>
        </form>
      </div>
    </section>

    <section v-else class="workspace">
      <aside class="workspace-sidebar">
        <div class="brand-card">
          <p class="eyebrow">mygo</p>
          <h2>聊天控制台</h2>
          <p class="brand-copy">你可以直接在这里完成注册后的整个聊天闭环。</p>
        </div>

        <div class="identity-card">
          <div>
            <strong>{{ currentUser.display_name || currentUser.username }}</strong>
            <span>@{{ currentUser.username }}</span>
          </div>
          <div class="identity-actions">
            <button type="button" class="ghost-btn" @click="copyUserId">复制用户 ID</button>
            <button type="button" class="ghost-btn" @click="logout">退出登录</button>
          </div>
        </div>

        <div class="panel-head">
          <div>
            <h3>会话列表</h3>
            <small>{{ conversations.length }} 个会话</small>
          </div>
          <button type="button" class="primary-btn primary-btn--small" @click="createModalOpen = true">
            新建会话
          </button>
        </div>

        <div class="conversation-list">
          <button
            v-for="conversation in conversations"
            :key="conversation.id"
            type="button"
            class="conversation-card"
            :class="{ 'conversation-card--active': conversation.id === activeConversationId }"
            @click="openConversation(conversation.id)"
          >
            <div class="conversation-card__main">
              <strong>{{ conversation.name }}</strong>
              <span>{{ conversation.member_role || 'member' }}</span>
            </div>
            <div class="conversation-card__meta">
              <small>{{ formatTime(conversation.updated_at) }}</small>
              <mark v-if="conversation.unread_count > 0">{{ conversation.unread_count }}</mark>
            </div>
          </button>

          <div v-if="!conversations.length" class="empty-slot">
            还没有会话。先创建一个测试群，把其他用户拉进来试试。
          </div>
        </div>
      </aside>

      <main class="workspace-main">
        <header class="workspace-toolbar">
          <div>
            <p class="eyebrow">实时状态</p>
            <h2>{{ activeConversation?.name || '请选择一个会话' }}</h2>
            <small v-if="activeConversation">
              角色：{{ activeConversation.member_role }} · 未读：{{ activeConversation.unread_count }}
            </small>
          </div>

          <div class="toolbar-actions">
            <span class="status-pill">{{ wsState }}</span>
            <button
              v-if="activeConversation && messages.length"
              type="button"
              class="ghost-btn"
              @click="markConversationRead()"
            >
              标记已读
            </button>
            <button type="button" class="ghost-btn" @click="loadConversations(true)">刷新</button>
          </div>
        </header>

        <p v-if="workspaceError" class="workspace-error">{{ workspaceError }}</p>

        <div v-if="!activeConversation" class="blank-panel">
          <div class="blank-card">
            <h3>先选一个会话，或者创建新的讨论组</h3>
            <p>你也可以先注册两个账号，在两个浏览器窗口里测试实时消息和未读变化。</p>
          </div>
        </div>

        <template v-else>
          <section ref="messageScroller" class="message-stream">
            <article
              v-for="message in messages"
              :key="message.id"
              class="message-row"
              :class="{ 'message-row--self': message.sender_id === currentUser.id }"
            >
              <div class="message-card">
                <header class="message-meta">
                  <strong>{{ senderLabel(message) }}</strong>
                  <span>{{ formatTime(message.created_at) }}</span>
                </header>
                <div class="message-body" v-html="renderMessageHtml(message.content)"></div>
              </div>
            </article>

            <div v-if="conversationBusy" class="empty-slot">正在加载会话内容…</div>
            <div v-else-if="!messages.length" class="empty-slot">
              这个会话还没有消息，发一句话开始吧。
            </div>
          </section>

          <footer class="composer-card">
            <textarea
              v-model="composer"
              class="composer-input"
              placeholder="输入消息，按 Enter 发送，Shift + Enter 换行"
              @keydown.enter.exact.prevent="sendMessage()"
            />

            <div class="composer-actions">
              <label class="ghost-btn ghost-btn--file">
                <input type="file" hidden @change="handleAttachmentUpload" />
                {{ uploadBusy ? '上传中…' : '上传附件' }}
              </label>
              <button type="button" class="primary-btn" :disabled="sendBusy" @click="sendMessage()">
                {{ sendBusy ? '发送中…' : '发送消息' }}
              </button>
            </div>
          </footer>
        </template>
      </main>

      <aside class="workspace-aside">
        <div class="aside-card">
          <div class="panel-head panel-head--tight">
            <div>
              <h3>成员面板</h3>
              <small>{{ members.length }} 名成员</small>
            </div>
          </div>

          <div v-if="activeConversation && canManageMembers" class="member-editor">
            <UserSearchSelect
              v-model="inviteUsers"
              :token="sessionToken"
              :exclude-ids="members.map((member) => member.user_id)"
              placeholder="搜索用户并加入当前会话"
            />
            <button type="button" class="primary-btn primary-btn--small" :disabled="inviteBusy" @click="addMembers">
              {{ inviteBusy ? '处理中…' : '添加成员' }}
            </button>
          </div>

          <div v-if="activeConversation" class="member-list">
            <article v-for="member in members" :key="member.user_id" class="member-card">
              <div>
                <strong>{{ member.display_name || member.username || shortId(member.user_id) }}</strong>
                <span>@{{ member.username || shortId(member.user_id) }}</span>
              </div>
              <small>{{ member.role }}</small>
            </article>

            <div v-if="memberBusy" class="empty-slot">正在加载成员…</div>
            <div v-else-if="!members.length" class="empty-slot">当前会话还没有成员数据。</div>
          </div>

          <div v-else class="empty-slot">
            选择会话后，这里会显示成员和权限信息。
          </div>
        </div>
      </aside>
    </section>

    <div v-if="createModalOpen" class="modal-backdrop" @click.self="createModalOpen = false">
      <div class="modal-card">
        <div class="panel-head">
          <div>
            <h3>创建新会话</h3>
            <small>搜索用户并把他们加入你的讨论组</small>
          </div>
          <button type="button" class="ghost-btn" @click="createModalOpen = false">关闭</button>
        </div>

        <label class="field">
          <span>会话名称</span>
          <input v-model="createForm.name" type="text" placeholder="例如：产品迭代小组" />
        </label>

        <UserSearchSelect
          v-model="createForm.selectedUsers"
          :token="sessionToken"
          :exclude-ids="[currentUser.id]"
          placeholder="搜索并选择要加入会话的成员"
        />

        <div class="modal-actions">
          <button type="button" class="ghost-btn" @click="createModalOpen = false">取消</button>
          <button type="button" class="primary-btn" :disabled="createBusy" @click="createConversation">
            {{ createBusy ? '创建中…' : '创建并进入会话' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
