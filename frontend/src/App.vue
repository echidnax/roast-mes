<script setup lang="ts">
import { ref } from 'vue'
import MachinesView from './views/MachinesView.vue'
import WorkOrdersView from './views/WorkOrdersView.vue'

const tab = ref<'machines' | 'orders'>('machines')
</script>

<template>
  <header class="topbar">
    <h1>烘豆機台 MES</h1>
    <nav>
      <button :class="{ active: tab === 'machines' }" @click="tab = 'machines'">機台監控</button>
      <button :class="{ active: tab === 'orders' }" @click="tab = 'orders'">工單</button>
    </nav>
  </header>
  <main>
    <MachinesView v-if="tab === 'machines'" />
    <WorkOrdersView v-else />
  </main>
</template>