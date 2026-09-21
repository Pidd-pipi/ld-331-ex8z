<template>
  <div>
    <div class="page-title">
      <div>
        <h1>调班与替班申请</h1>
        <p>主管审批时按最新排班规则复查；存在冲突则整次拒绝，原班表与申请状态保持不变。</p>
      </div>
    </div>
    <el-card>
      <el-table :data="rows">
        <el-table-column prop="id" label="#" width="70"/>
        <el-table-column label="申请人">
          <template #default="scope">{{ scope.row.applicant?.name }}</template>
        </el-table-column>
        <el-table-column label="替班人">
          <template #default="scope">{{ scope.row.substitute?.name }}</template>
        </el-table-column>
        <el-table-column prop="reason" label="调班原因"/>
        <el-table-column prop="status" label="状态" width="110">
          <template #default="scope">
            <el-tag :type="statusType(scope.row.status)">{{ statusText(scope.row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column v-if="auth.role !== 'staff'" label="操作" width="180">
          <template #default="scope">
            <el-button size="small" type="success" :disabled="scope.row.status !== 'pending'"
                       @click="review(scope.row, true)">同意</el-button>
            <el-button size="small" :disabled="scope.row.status !== 'pending'"
                       @click="review(scope.row, false)">驳回</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import {onMounted, ref} from 'vue'
import {ElMessage} from 'element-plus'
import api from '../api/http'
import {auth} from '../stores/auth'

const rows = ref<any[]>([])

async function load() {
  rows.value = await api.get('/shift-requests')
}

async function review(row: any, approved: boolean) {
  try {
    await api.put(`/shift-requests/${row.id}/review`, {approved})
    ElMessage.success('审批已完成')
    await load()
  } catch (error: any) {
    // 冲突拒绝：服务端未改动任何数据，重新回读以确认状态仍为待审批。
    ElMessage.error(error.message)
    await load()
  }
}

function statusText(status: string) {
  return {pending: '待审批', approved: '已通过', rejected: '已驳回'}[status] || status
}

function statusType(status: string) {
  return ({pending: 'warning', approved: 'success', rejected: 'info'} as Record<string, string>)[status] || ''
}

onMounted(load)
</script>
