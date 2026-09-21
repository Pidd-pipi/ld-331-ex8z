<template>
  <div>
    <div class="page-title">
      <div>
        <h1>可视化排班表</h1>
        <p>按规则一键生成，也可直接手动调整班次；调整前会按最新规则复核，冲突将整次拒绝。</p>
      </div>
      <el-button type="primary" @click="generate">一键生成本周</el-button>
    </div>
    <el-card>
      <div class="filters">
        <el-select v-model="department" placeholder="科室" style="width:180px">
          <el-option v-for="item in departments" :key="item.id" :value="item.id" :label="item.name"/>
        </el-select>
        <el-date-picker v-model="range" type="daterange" value-format="YYYY-MM-DD"/>
        <el-select v-model="staffFilter" placeholder="全部人员" clearable style="width:180px">
          <el-option v-for="item in staffs" :key="item.id" :value="item.id" :label="item.name"/>
        </el-select>
        <el-select v-model="conflictFilter" placeholder="冲突状态" clearable style="width:150px">
          <el-option label="全部" :value="''"/>
          <el-option label="仅看冲突" value="conflict"/>
          <el-option label="仅看正常" value="ok"/>
        </el-select>
        <el-button @click="load">筛选</el-button>
        <el-button text type="primary" @click="load">刷新</el-button>
        <el-tag v-if="conflictCount>0" type="danger" disable-transitions>本视图 {{ conflictCount }} 条冲突</el-tag>
      </div>
      <ScheduleLegend/>
      <el-table :data="visibleSchedules" :row-class-name="rowClass">
        <el-table-column label="日期" width="120"><template #default="scope">{{ scope.row.work_date.slice(0,10) }}</template></el-table-column>
        <el-table-column label="人员" width="120"><template #default="scope">{{ scope.row.staff?.name }}</template></el-table-column>
        <el-table-column label="班次" width="170">
          <template #default="scope">
            <el-select v-model="scope.row.shift_id" size="small" @change="update(scope.row)">
              <el-option v-for="shift in shifts" :key="shift.id" :value="shift.id" :label="shift.name"/>
            </el-select>
          </template>
        </el-table-column>
        <el-table-column label="冲突原因">
          <template #default="scope">
            <el-tooltip v-if="scope.row.has_conflict" :content="scope.row.conflict_reasons" placement="top" :show-after="100">
              <el-tag type="danger" size="small" class="conflict-tag">{{ scope.row.conflict_reasons }}</el-tag>
            </el-tooltip>
            <el-tag v-else type="success" size="small" effect="plain">正常</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="note" label="备注"/>
      </el-table>
    </el-card>
  </div>
</template>
<script setup lang="ts">
import {computed, onMounted, ref} from 'vue';
import {ElMessage} from 'element-plus';
import api from '../api/http';
import ScheduleLegend from '../components/ScheduleLegend.vue'

const departments = ref<any[]>([])
const staffs = ref<any[]>([])
const schedules = ref<any[]>([])
const shifts = ref([{id:1,name:'白班'},{id:2,name:'中班'},{id:3,name:'夜班'},{id:4,name:'休息'}])
const department = ref(1)
const staffFilter = ref<number | undefined>(undefined)
const conflictFilter = ref('')
const range = ref<string[]>([new Date().toISOString().slice(0,10), new Date(Date.now()+6*86400000).toISOString().slice(0,10)])

async function load() {
  // 服务端按条件过滤，冲突标记由后端持久化，筛选、刷新后重新回读仍在。
  schedules.value = await api.get('/schedules', {
    params: {department_id: department.value, staff_id: staffFilter.value || 0, from: range.value[0], to: range.value[1]},
  })
}
const visibleSchedules = computed(() => schedules.value.filter(row => {
  if (conflictFilter.value === 'conflict') return !!row.has_conflict
  if (conflictFilter.value === 'ok') return !row.has_conflict
  return true
}))
const conflictCount = computed(() => schedules.value.filter(row => row.has_conflict).length)

async function generate() {
  try {
    const res: any = await api.post('/schedules/generate', {department_id: department.value, start_date: range.value[0], end_date: range.value[1]})
    ElMessage.success(`排班已生成，共 ${res.created} 条`)
    await load()
  } catch (error: any) {
    ElMessage.error(error.message)
  }
}
async function update(row: any) {
  try {
    await api.put(`/schedules/${row.id}`, {shift_id: row.shift_id, note: row.note || ''})
    ElMessage.success('班次已调整')
    await load()
  } catch (error: any) {
    // 冲突时整次拒绝：重新回读，下拉显示恢复为原班表。
    ElMessage.error(error.message)
    await load()
  }
}
function rowClass({row}: {row: any}) {
  return row.has_conflict ? 'shift-conflict' : `shift-${row.shift?.kind || ''}`
}
onMounted(async () => {
  departments.value = await api.get('/departments')
  staffs.value = await api.get('/staff', {params: {department_id: department.value}})
  await load()
})
</script>
