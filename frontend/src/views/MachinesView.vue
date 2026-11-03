<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { api } from '../api'
import type { Machine } from '../types'

const machines = ref<Machine[]>([])
const error = ref('')
let timer: number | undefined

async function load() {
  try {
    machines.value = await api.machines()
    error.value = ''
  } catch (e) {
    error.value = e instanceof Error ? e.message : '無法取得資料'
  }
}

// 畫面打開時抓一次,之後每 2 秒抓一次
onMounted(() => {
  load()
  timer = window.setInterval(load, 2000)
})
onUnmounted(() => window.clearInterval(timer))

const STATE_NAMES = ['待機', '烘焙中', '完成', '故障']
const STATE_CLASSES = ['idle', 'roasting', 'done', 'fault']

function stateName(m: Machine) {
  if (!m.online) return '離線'
  return STATE_NAMES[m.values.cycle_state ?? 0] ?? '待機'
}

function stateClass(m: Machine) {
  if (!m.online) return 'offline'
  return STATE_CLASSES[m.values.cycle_state ?? 0] ?? 'idle'
}

// 數字轉文字,沒有資料就顯示 —
function show(v: number | undefined, digits = 1) {
  return v === undefined ? '—' : v.toFixed(digits)
}

// 秒數轉成 分:秒
function clock(sec: number | undefined) {
  if (sec === undefined) return '—'
  const m = Math.floor(sec / 60)
  const s = Math.round(sec % 60)
  return `${m}:${String(s).padStart(2, '0')}`
}
</script>

<template>
  <p v-if="error" class="banner">{{ error }}(請確認後端有在執行)</p>

  <section class="grid">
    <article v-for="m in machines" :key="m.id" class="card" :class="stateClass(m)">
      <header>
        <h2>{{ m.name }}</h2>
        <span class="chip" :class="stateClass(m)">{{ stateName(m) }}</span>
      </header>

      <p class="big">{{ show(m.values.temperature) }}<small> °C</small></p>

      <dl>
        <div><dt>風門開度</dt><dd>{{ show(m.values.airflow, 0) }} %</dd></div>
        <div>
          <dt>剩餘時間</dt>
          <dd>{{ stateClass(m) === 'roasting' ? clock(m.values.remaining_sec) : '—' }}</dd>
        </div>
        <div><dt>目標溫度</dt><dd>{{ show(m.values.set_temperature, 0) }} °C</dd></div>
      </dl>
    </article>
  </section>
</template>