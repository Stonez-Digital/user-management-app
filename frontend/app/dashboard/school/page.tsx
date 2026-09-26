"use client";
import {useEffect,useState} from "react";
import Link from "next/link";
import {useRouter} from "next/navigation";

type School={id:string;name:string;code:string;slug:string;logo_url?:string|null;status:string;created_at:string;updated_at:string};

async function api(path:string,init?:RequestInit){
 const t=localStorage.getItem("access_token");
 const headers=new Headers(init?.headers);
 headers.set("Authorization","Bearer "+t);
 if(!(init?.body instanceof FormData)) headers.set("Content-Type","application/json");
 const r=await fetch("/backend"+path,{...init,headers});
 const d=await r.json().catch(()=>({}));
 if(!r.ok) throw Error(d?.error?.message||d?.error||"Request failed");
 return d;
}

export default function SchoolSettings(){
 const router=useRouter();
 const[school,setSchool]=useState<School|null>(null);
 const[name,setName]=useState("");
 const[description,setDescription]=useState("");const[address,setAddress]=useState("");const[contactEmail,setContactEmail]=useState("");const[contactPhone,setContactPhone]=useState("");const[websiteURL,setWebsiteURL]=useState("");
 const[selectedLogo,setSelectedLogo]=useState<File|null>(null);
 const[previewURL,setPreviewURL]=useState("");
 const[status,setStatus]=useState("");
 const[error,setError]=useState("");
 const[saving,setSaving]=useState(false);
 const[uploading,setUploading]=useState(false);

 useEffect(()=>{api("/me").then(me=>{if(me?.role!=="school_admin"){router.replace(me?.role==="super_admin"?"/platform/schools":"/dashboard");return} return api("/admin/school")}).then(s=>{if(s){setSchool(s);setName(s.name);setDescription(s.description||"");setAddress(s.address||"");setContactEmail(s.contact_email||"");setContactPhone(s.contact_phone||"");setWebsiteURL(s.website_url||"");setPreviewURL(s.logo_url||"");setStatus(s.status)}}).catch(e=>{setError(e.message);if(e.message==="Session expired")router.replace("/")})},[router]);

 useEffect(()=>()=>{if(previewURL.startsWith("blob:"))URL.revokeObjectURL(previewURL)},[previewURL]);

 function selectLogo(file:File|null){
  setSelectedLogo(file);
  if(!file)return;
  if(previewURL.startsWith("blob:"))URL.revokeObjectURL(previewURL);
  setPreviewURL(URL.createObjectURL(file));
  setError("");
 }

 async function save(e:React.FormEvent){
  e.preventDefault();setSaving(true);setError("");
  try{
   const s=await api("/admin/school",{method:"PUT",body:JSON.stringify({name,logo_url:school?.logo_url||"",description,address,contact_email:contactEmail,contact_phone:contactPhone,website_url:websiteURL})});
   setSchool(s);setName(s.name);setPreviewURL(s.logo_url||"");setStatus(s.status);
  }catch(e){setError(e instanceof Error?e.message:"Unable to save school")}finally{setSaving(false)}
 }

 async function uploadLogo(){
  if(!selectedLogo)return;
  setUploading(true);setError("");
  try{
   const form=new FormData();form.append("logo",selectedLogo);
   const s=await api("/admin/school/logo",{method:"POST",body:form});
   setSchool(s);setPreviewURL(s.logo_url||"");setSelectedLogo(null);
  }catch(e){setError(e instanceof Error?e.message:"Unable to upload school logo")}finally{setUploading(false)}
 }

 return <div className="app-shell"><aside className="sidebar"><div className="logo"><span>S</span><div><strong>Stonez</strong><small>School OS</small></div></div><nav><Link href="/dashboard">Overview</Link><Link className="active" href="/dashboard/school">School setup</Link><Link href="/dashboard/content">Blog & Gallery</Link><Link href="/dashboard/academic">Academic</Link><Link href="/dashboard/users">Users & Roles</Link><Link href="/dashboard/audit">Audit Logs</Link></nav></aside>
 <main className="content"><Link className="back" href="/dashboard">← Back to overview</Link><header className="topbar"><div><p className="eyebrow">TENANT CONFIGURATION</p><h1>School setup</h1><p className="muted">Configure the identity of the school currently signed in.</p></div>{school&&<div className="status"><span/>{status}</div>}</header>
 {error&&<div className="error banner">{error}</div>}
 <section className="panel"><div className="panel-head"><div><h2>School workspace</h2><p>Each school is an isolated tenant with its own users, academic records and operations.</p></div></div>
 {school&&<div className="school-identity"><div className="school-code-card"><small>School code</small><strong>{school.code}</strong><span>Use this code when a school requires tenant confirmation at sign-in.</span></div><div className="school-code-card"><small>Tenant ID</small><strong>{school.id.slice(0,8)}…</strong><span>Internal identifier used for tenant isolation.</span></div></div>}
 <form onSubmit={save} className="form-grid school-form"><label>School name<input value={name} onChange={e=>setName(e.target.value)} placeholder="e.g. Stonez Digital Academy" minLength={2} maxLength={160} required/></label>
 <div className="school-logo-upload"><label>School logo<input type="file" accept="image/png,image/jpeg,image/webp" onChange={e=>selectLogo(e.target.files?.[0]||null)}/><small className="muted">PNG, JPEG or WebP. Maximum 2 MB.</small></label>{previewURL&&<div className="logo-preview"><img src={previewURL} alt={school?.name+" logo preview"}/></div>}{selectedLogo&&<button type="button" onClick={uploadLogo} disabled={uploading}>{uploading?"Uploading…":"Upload school logo"}</button>}</div>
 <div className="form-action"><button disabled={saving}>{saving?"Saving…":"Save school profile"}</button></div></form><div className="tenant-note"><strong>School login</strong><span>/school/{school?.slug||"your-school"}/login</span></div></section>
 <section className="panel"><div className="panel-head"><div><h2>Multi-school setup model</h2><p>Use a separate school tenant for every institution you onboard.</p></div></div><div className="tenant-cards"><div><strong>School A</strong><span>Example: Bright Future College</span><small>Code: BFC-001</small></div><div><strong>School B</strong><span>Example: Community Secondary School</span><small>Code: CSS-002</small></div><div><strong>School C</strong><span>Example: Stonez Academy</span><small>Code: STA-003</small></div></div><div className="tenant-next"><strong>Next onboarding step</strong><span>New school creation should be performed by the platform administrator so a tenant, initial school administrator and isolated workspace are created together.</span></div></section>
 </main></div>
}