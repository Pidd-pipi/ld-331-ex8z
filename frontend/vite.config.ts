import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
export default defineConfig({ plugins:[vue()], server:{port:18931, host:'0.0.0.0', proxy:{'/api':{target:'http://localhost:19931',changeOrigin:true}}} })
