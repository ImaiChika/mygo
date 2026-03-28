<script setup>
import { computed, ref, watch } from 'vue'
import { apiRequest, shortId } from '../lib/api'

const props = defineProps({
  token: {
    type: String,
    required: true
  },
  modelValue: {
    type: Array,
    default: () => []
  },
  placeholder: {
    type: String,
    default: '输入用户名、邮箱或显示名搜索'
  },
  disabled: {
    type: Boolean,
    default: false
  },
  excludeIds: {
    type: Array,
    default: () => []
  }
})

const emit = defineEmits(['update:modelValue'])

const query = ref('')
const loading = ref(false)
const error = ref('')
const results = ref([])
let timer = null

const selectedIds = computed(() => new Set(props.modelValue.map((user) => user.id)))

watch(
  query,
  (value) => {
    clearTimeout(timer)
    results.value = []
    error.value = ''

    if (!value || value.trim().length < 2) {
      return
    }

    timer = setTimeout(async () => {
      try {
        loading.value = true
        const users = await apiRequest(`/users/search?q=${encodeURIComponent(value.trim())}`, {
          token: props.token
        })
        results.value = users.filter(
          (user) => !selectedIds.value.has(user.id) && !props.excludeIds.includes(user.id)
        )
      } catch (err) {
        error.value = err.message
      } finally {
        loading.value = false
      }
    }, 280)
  },
  { flush: 'post' }
)

function appendUser(user) {
  emit('update:modelValue', [...props.modelValue, user])
  query.value = ''
  results.value = []
}

function removeUser(userId) {
  emit(
    'update:modelValue',
    props.modelValue.filter((user) => user.id !== userId)
  )
}
</script>

<template>
  <div class="search-select">
    <div class="selected-chips" v-if="modelValue.length">
      <button
        v-for="user in modelValue"
        :key="user.id"
        type="button"
        class="chip"
        @click="removeUser(user.id)"
      >
        <span>{{ user.display_name || user.username }}</span>
        <small>{{ shortId(user.id) }}</small>
      </button>
    </div>

    <input
      v-model="query"
      :disabled="disabled"
      :placeholder="placeholder"
      class="search-input"
      type="text"
    />

    <div class="search-feedback">
      <span v-if="loading">正在搜索用户…</span>
      <span v-else-if="error">{{ error }}</span>
      <span v-else-if="query && query.trim().length < 2">至少输入 2 个字符</span>
    </div>

    <div v-if="results.length" class="search-results">
      <button
        v-for="user in results"
        :key="user.id"
        type="button"
        class="search-result"
        @click="appendUser(user)"
      >
        <strong>{{ user.display_name || user.username }}</strong>
        <span>@{{ user.username }}</span>
        <small>{{ user.email }}</small>
      </button>
    </div>
  </div>
</template>
