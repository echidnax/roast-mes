<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { api } from '../api'
import type { Machine, WorkOrder, WorkOrderStatus } from '../types'

const orders = ref<WorkOrder[]>([])
const machines = ref<Machine[]>([])
const message = ref('')
const isError = ref(false)

// 「新增工單」表單的內容
const form = reactive({ order_no: '', product_spec: '', planned_qty: 10 })
// 每張工單各自選了哪台機器、要回報多少數量
const pick = reactive<Record<number, string>>({})
const qty = reactive<Record<number, number>>({})

const LABEL: Record<WorkOrderStatus, string> = {
  created: '待派工',
  released: '已派工',
  running: '生產中',
  completed: '已完工',
  cancelled: '已取消',
}

async function load() {
  orders.value = await api.workOrders()
}

// 所有按鈕共用:先做事,成功顯示成功訊息,失敗顯示原因,最後都重新載入最新資料
async function run(action: () => Promise<unknown>, okText: string) {
  try {
    await action()
    message.value = okText
    isError.value = false
  } catch (e) {
    message.value = e instanceof Error ? e.message : '操作失敗,請稍後再試'
    isError.value = true
  }
  await load()
}

function create() {
  run(async () => {
    await api.createWorkOrder(form.order_no, form.product_spec, Number(form.planned_qty))
    form.order_no = ''
    form.product_spec = ''
  }, '工單已建立')
}

function move(wo: WorkOrder, to: WorkOrderStatus, machine = '') {
  run(() => api.transition(wo, to, machine), `${wo.order_no} 已改為「${LABEL[to]}」`)
}

function report(wo: WorkOrder) {
  run(() => api.report(wo, Number(qty[wo.id]) || 0), `${wo.order_no} 已回報產量`)
}

onMounted(async () => {
  try {
    machines.value = await api.machines()
    await load()
  } catch {
    message.value = '無法載入資料,請確認後端有在執行'
    isError.value = true
  }
})
</script>

<template>
  <p v-if="message" class="banner" :class="{ ok: !isError }" role="status">{{ message }}</p>

  <section class="panel">
    <h2>新增工單</h2>
    <form class="row" @submit.prevent="create">
      <label>工單編號<input v-model="form.order_no" required placeholder="WO-101" /></label>
      <label>商品規格<input v-model="form.product_spec" required placeholder="衣索比亞 淺焙" /></label>
      <label>計畫數量<input v-model.number="form.planned_qty" type="number" min="1" required /></label>
      <button type="submit">建立工單</button>
    </form>
  </section>

  <section class="panel">
    <h2>工單列表</h2>
    <p v-if="orders.length === 0" class="empty">還沒有工單,從上面建立第一張。</p>

    <div v-else class="table-wrap">
      <table>
        <thead>
          <tr><th>編號</th><th>規格</th><th>機台</th><th>進度</th><th>狀態</th><th>操作</th></tr>
        </thead>
        <tbody>
          <tr v-for="wo in orders" :key="wo.id">
            <td>{{ wo.order_no }}</td>
            <td>{{ wo.product_spec }}</td>
            <td>{{ wo.machine_id || '—' }}</td>
            <td>
              <progress :value="wo.produced_qty" :max="wo.planned_qty"></progress>
              {{ wo.produced_qty }} / {{ wo.planned_qty }}
            </td>
            <td><span class="chip" :class="wo.status">{{ LABEL[wo.status] }}</span></td>
            <td class="actions">
              <!-- 待派工:選機台後派工 -->
              <template v-if="wo.status === 'created'">
                <select v-model="pick[wo.id]">
                  <option value="" disabled>選擇機台</option>
                  <option v-for="m in machines" :key="m.id" :value="m.id">{{ m.name }}</option>
                </select>
                <button :disabled="!pick[wo.id]" @click="move(wo, 'released', pick[wo.id])">派工</button>
              </template>

              <!-- 已派工:開工 -->
              <button v-if="wo.status === 'released'" @click="move(wo, 'running')">開工</button>

              <!-- 生產中:回報產量、完工 -->
              <template v-if="wo.status === 'running'">
                <input v-model.number="qty[wo.id]" type="number" min="1" placeholder="數量" class="qty" />
                <button @click="report(wo)">回報產量</button>
                <button :disabled="wo.produced_qty < wo.planned_qty" @click="move(wo, 'completed')">完工</button>
              </template>

              <!-- 還沒結束的工單都可以取消 -->
              <button
                v-if="['created', 'released', 'running'].includes(wo.status)"
                class="ghost"
                @click="move(wo, 'cancelled')"
              >取消</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>