<template>
  <div>
    <div class="page-title">
      <div>
        <h1>可视化排班表</h1>
        <p>按规则一键生成，也可直接手动调整班次；违反规则的调整会被整次拒绝。</p>
      </div>
      <el-button type="primary" :loading="generating" @click="generate">一键生成本周</el-button>
    </div>
    <el-card>
      <div class="filters">
        <el-select v-model="department" placeholder="科室" style="width:180px">
          <el-option v-for="item in departments" :key="item.id" :value="item.id" :label="item.name"/>
        </el-select>
        <el-date-picker v-model="range" type="daterange" value-format="YYYY-MM-DD"/>
        <el-checkbox v-model="conflictOnly">仅看冲突</el-checkbox>
        <el-button @click="load">筛选</el-button>
        <el-button v-if="conflictCount > 0" type="danger" plain>{{ conflictCount }} 条冲突</el-button>
      </div>
      <ScheduleLegend/>
      <el-table :data="filteredSchedules" :row-class-name="rowClass">
        <el-table-column label="日期" width="120">
          <template #default="scope">{{ scope.row.work_date.slice(0, 10) }}</template>
        </el-table-column>
        <el-table-column label="人员" width="120">
          <template #default="scope">{{ scope.row.staff?.name }}</template>
        </el-table-column>
        <el-table-column label="班次" width="140">
          <template #default="scope">
            <el-select v-model="scope.row.shift_id" size="small" @change="update(scope.row)">
              <el-option v-for="shift in shifts" :key="shift.id" :value="shift.id" :label="shift.name"/>
            </el-select>
          </template>
        </el-table-column>
        <el-table-column prop="note" label="备注" width="180"/>
        <el-table-column label="冲突原因">
          <template #default="scope">
            <span v-if="scope.row.conflict_reason" class="conflict-text">
              <el-icon><WarningFilled/></el-icon>{{ scope.row.conflict_reason }}
            </span>
            <span v-else class="conflict-ok">无</span>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import {computed, onMounted, ref} from 'vue'
import {ElMessage} from 'element-plus'
import {WarningFilled} from '@element-plus/icons-vue'
import api from '../api/http'
import ScheduleLegend from '../components/ScheduleLegend.vue'

const departments = ref<any[]>([])
const schedules = ref<any[]>([])
const shifts = ref([
  {id: 1, name: '白班'},
  {id: 2, name: '中班'},
  {id: 3, name: '夜班'},
  {id: 4, name: '休息'},
])
const department = ref(1)
const today = new Date().toISOString().slice(0, 10)
const inAWeek = new Date(Date.now() + 6 * 86400000).toISOString().slice(0, 10)
const range = ref<string[]>([today, inAWeek])
const conflictOnly = ref(false)
const generating = ref(false)

const conflictCount = computed(() => schedules.value.filter((row: any) => row.conflict_reason).length)
const filteredSchedules = computed(() =>
  conflictOnly.value ? schedules.value.filter((row: any) => row.conflict_reason) : schedules.value,
)

async function load() {
  schedules.value = await api.get('/schedules', {
    params: {department_id: department.value, from: range.value[0], to: range.value[1]},
  })
}

async function generate() {
  generating.value = true
  try {
    await api.post('/schedules/generate', {
      department_id: department.value,
      start_date: range.value[0],
      end_date: range.value[1],
    })
    ElMessage.success('排班已生成')
    await load()
  } catch (error: any) {
    ElMessage.error(error.message)
  } finally {
    generating.value = false
  }
}

async function update(row: any) {
  try {
    await api.put(`/schedules/${row.id}`, {shift_id: row.shift_id, note: row.note || ''})
    ElMessage.success('班次已调整')
    await load()
  } catch (error: any) {
    ElMessage.error(error.message)
    await load()
  }
}

function rowClass({row}: {row: any}) {
  const classes = [`shift-${row.shift?.kind || ''}`]
  if (row.conflict_reason) classes.push('schedule-conflict')
  return classes.join(' ')
}

onMounted(async () => {
  departments.value = await api.get('/departments')
  await load()
})
</script>
