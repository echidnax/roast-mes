<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { api } from '../api'
import type { Alarm, Machine } from '../types'

const machines = ref<Machine[]>([])
const alarms = ref<Alarm[]>([])
const error = ref('')
let timer: number | undefined

async function load() {
  try {
    const [m, a] = await Promise.all([api.machines(), api.alarms()])
    machines.value = m
    alarms.value = a
    error.value = ''
  } catch (e) {
    error.value = e instanceof Error ? e.message : '無法取得資料'
  }
}

onMounted(() => {
  load()
  timer = window.setInterval(load, 2000)
})
onUnmounted(() => window.clearInterval(timer))

// 目前還在響的警報:每個「機台+種類」只看最新一筆,是「發生」就代表還沒解除
const activeAlarms = computed(() => {
  const seen = new Set<string>()
  const list: Alarm[] = []
  for (const a of alarms.value) {
    const key = a.machine_id + '/' + a.kind
    if (seen.has(key)) continue
    seen.add(key)
    if (a.active) list.push(a)
  }
  return list
})

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

function show(v: number | undefined, digits = 1) {
  return v === undefined ? '—' : v.toFixed(digits)
}

function clock(sec: number | undefined) {
  if (sec === undefined) return '—'
  const m = Math.floor(sec / 60)
  const s = Math.round(sec % 60)
  return `${m}:${String(s).padStart(2, '0')}`
}

function time(iso: string) {
  return new Date(iso).toLocaleTimeString('zh-TW', { hour12: false })
}
</script>

<template>
  <p v-if="error" class="banner">{{ error }}(請確認後端有在執行)</p>

  <div v-if="activeAlarms.length" class="alarm-bar" role="alert">
    <strong>目前有 {{ activeAlarms.length }} 項警報</strong>
    <ul>
      <li v-for="a in activeAlarms" :key="a.id">{{ a.machine_id }}:{{ a.message }}</li>
    </ul>
  </div>

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

  <section class="panel">
    <h2>警報紀錄</h2>
    <p v-if="alarms.length === 0" class="empty">目前沒有警報。</p>
    <ul v-else class="alarm-list">
      <li v-for="a in alarms" :key="a.id">
        <time>{{ time(a.occurred_at) }}</time>
        <strong>{{ a.machine_id }}</strong>
        <span>{{ a.message }}</span>
        <span class="chip" :class="a.active ? 'fault' : 'done'">{{ a.active ? '發生' : '解除' }}</span>
      </li>
    </ul>
  </section>
</template>