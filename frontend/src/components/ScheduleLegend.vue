<template>
  <div class="legend"><span v-for="item in items" :key="item.name"><i :style="{ backgroundColor: item.color }"></i>{{ item.name }}</span></div>
</template>
<script setup lang="ts">
const items = [{ name: '白班', color: '#409EFF' }, { name: '中班', color: '#E6A23C' }, { name: '夜班', color: '#722ED1' }, { name: '休息', color: '#909399' }]
</script>
