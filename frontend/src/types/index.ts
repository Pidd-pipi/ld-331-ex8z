export interface ApiResponse<T>{code:number;message:string;data:T}
export interface User{token:string;name:string;role:string;staff_id:number}
export interface Schedule{id:number;department_id:number;staff_id:number;shift_id:number;work_date:string;note:string;staff:{name:string};shift:{name:string;kind:string;color:string;hours:number}}
export interface Department{id:number;name:string;positions?:{id:number;name:string;staffing_quota:number;skills:string}[]}
