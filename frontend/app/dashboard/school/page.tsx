"use client";
import {useEffect,useState} from "react";
import Link from "next/link";
import {useRouter} from "next/navigation";

type School={id:string;name:string;code:string;status:string;created_at:string;updated_at:string};

async function api(path:string,init?:RequestInit){
 const t=localStorage.getItem("access_token");
 const r=await fetch("/backend"+path,{...init,headers:{"Content-Type":"application/json",Authorization:"Bearer "+t,...(init?.headers||{})}});
 const d=await r.json().catch(()=>({}));
 if(!r.ok) throw Error(d?.error?.message||d?.error||"Request failed");
 return d;
}

export default function SchoolSettings(){
 const router=useRouter();
 const[school,setSchool]=useState<School|null>(null);
 const[name,setName]=useState("");
 const[status,setStatus]=useState("");
 const[error,setError]=useState("");
 const[saving,setSaving]=useState(false);

 useEffect(()=>{api("/me").then(me=>{if(me?.role!=="school_admin"){router.replace(me?.role==="super_admin"?"/platform/schools":"/dashboard");return} return api("/admin/school")}).then(s=>{if(s){setSchool(s);setName(s.name);setStatus(s.status)}}).catch(e=>{setError(e.message);if(e.message==="Session expired")router.replace("/")})},[router]);

 async function save(e:React.FormEvent){e.preventDefault();setSaving(true);setError("");try{const s=await api("/admin/school",{method:"PUT",body:JSON.stringify({name})});setSchool(s);setName(s.name);setStatus(s.status)}catch(e){setError(e instanceof Error?e.message:"Unable to save school")}finally{setSaving(false)}}

 return <div className="app-shell"><aside className="sidebar"><div className="logo"><span>S</span><div><strong>Stonez</strong><small>School OS</small></div></div><nav><Link href="/dashboard">Overview</Link><Link className="active" href="/dashboard/school">School setup</Link><Link href="/dashboard/academic">Academic</Link><Link href="/dashboard/users">Users & Roles</Link><Link href="/dashboard/audit">Audit Logs</Link></nav></aside>
 <main className="content"><Link className="back" href="/dashboard">← Back to overview</Link><header className="topbar"><div><p className="eyebrow">TENANT CONFIGURATION</p><h1>School setup</h1><p className="muted">Configure the identity of the school currently signed in.</p></div>{school&&<div className="status"><span/>{status}</div>}</header>
 {error&&<div className="error banner">{error}</div>}
 <section className="panel"><div className="panel-head"><div><h2>School workspace</h2><p>Each school is an isolated tenant with its own users, academic records and operations.</p></div></div>
 {school&&<div className="school-identity"><div className="school-code-card"><small>School code</small><strong>{school.code}</strong><span>Use this code when a school requires tenant confirmation at sign-in.</span></div><div className="school-code-card"><small>Tenant ID</small><strong>{school.id.slice(0,8)}…</strong><span>Internal identifier used for tenant isolation.</span></div></div>}
 <form onSubmit={save} className="form-grid school-form"><label>School name<input value={name} onChange={e=>setName(e.target.value)} placeholder="e.g. Stonez Digital Academy" minLength={2} maxLength={160} required/></label><div className="form-action"><button disabled={saving}>{saving?"Saving…":"Save school profile"}</button></div></form></section>
 <section className="panel"><div className="panel-head"><div><h2>Multi-school setup model</h2><p>Use a separate school tenant for every institution you onboard.</p></div></div><div className="tenant-cards"><div><strong>School A</strong><span>Example: Bright Future College</span><small>Code: BFC-001</small></div><div><strong>School B</strong><span>Example: Community Secondary School</span><small>Code: CSS-002</small></div><div><strong>School C</strong><span>Example: Stonez Academy</span><small>Code: STA-003</small></div></div><div className="tenant-next"><strong>Next onboarding step</strong><span>New school creation should be performed by the platform administrator so a tenant, initial school administrator and isolated workspace are created together.</span></div></section>
 </main></div>
}