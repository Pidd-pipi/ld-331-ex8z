import { reactive } from 'vue'
export const auth=reactive({token:localStorage.getItem('gbsched_token')||'',name:localStorage.getItem('gbsched_name')||'',role:localStorage.getItem('gbsched_role')||'',staffId:Number(localStorage.getItem('gbsched_staff_id')||0)})
export function saveAuth(v:any){auth.token=v.token;auth.name=v.name;auth.role=v.role;auth.staffId=v.staff_id;Object.entries({gbsched_token:v.token,gbsched_name:v.name,gbsched_role:v.role,gbsched_staff_id:String(v.staff_id)}).forEach(([k,x])=>localStorage.setItem(k,x))}
export function logout(){['gbsched_token','gbsched_name','gbsched_role','gbsched_staff_id'].forEach(k=>localStorage.removeItem(k));auth.token='';auth.name='';auth.role='';auth.staffId=0}
