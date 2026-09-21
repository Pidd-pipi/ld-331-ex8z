import axios from 'axios'
const api=axios.create({baseURL:'/api/v1'})
api.interceptors.request.use(c=>{const t=localStorage.getItem('gbsched_token');if(t)c.headers.Authorization=`Bearer ${t}`;return c})
api.interceptors.response.use(r=>{if(r.data.code!==0)return Promise.reject(new Error(r.data.message));return r.data.data},e=>Promise.reject(new Error(e.response?.data?.message||'网络请求失败')))
export default api
