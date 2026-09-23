"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";

type Job={id:string;kind:string;file_name:string;status:string;total:number;valid:number;created:number;updated:number;skipped:number;failed:number;send_credentials:boolean;credentials_sent:number;credential_emails_failed:number;created_at:string;updated_at:string};
type ErrorRow={row:number;field?:string;message:string};
type Preview={job:Job;preview:Record<string,string>[];errors:ErrorRow[]};

async function api(path:string,options:RequestInit={}) {
 const token=localStorage.getItem("access_token");
 const r=await fetch("/backend"+path,{...options,headers:{Authorization:"Bearer "+token,...(options.headers||{})}});
 const text=await r.text();let d:any={};try{d=text?JSON.parse(text):{}}catch{d={error:text}}
 if(r.status===401)throw Error("Session expired");
 if(!r.ok)throw Error(d?.error?.message||d?.error||"Request failed");
 return d;
}

const templates:Record<string,string>={
 students:"admission_number,first_name,last_name,other_names,date_of_birth,gender,email,phone,class,section,session,student_status,parent_identifier\nBED-001,Jane,Doe,,,female,jane@example.com,08000000000,JSS1,A,2026/2027,active,PARENT-001\n",
 teachers:"staff_id,first_name,last_name,other_names,email,phone,gender,employment_date,department,designation,subjects,classes,status\nT-001,John,Doe,,john@example.com,08000000000,male,2026-09-01,Science,Teacher,Mathematics,JSS1,active\n",
 parents:"parent_identifier,first_name,last_name,other_names,relationship,phone,email,address,occupation,student_admission_number,primary\nPARENT-001,Jane,Doe,,mother,08000000000,parent@example.com,,Trader,BED-001,true\n"
};

function downloadTemplate(kind:string){const blob=new Blob([templates[kind]],{type:"text/csv;charset=utf-8"});const a=document.createElement("a");a.href=URL.createObjectURL(blob);a.download=kind+"-import-template.csv";a.click();URL.revokeObjectURL(a.href)}

export default function OnboardingPage(){
 const [kind,setKind]=useState("students");const [preview,setPreview]=useState<Preview|null>(null);const [jobs,setJobs]=useState<Job[]>([]);
 const [busy,setBusy]=useState(false);const [sendCredentials,setSendCredentials]=useState(true);const [error,setError]=useState("");const [message,setMessage]=useState("");const fileRef=useRef<HTMLInputElement>(null);
 async function loadJobs(){try{const d=await api("/admin/onboarding/bulk");setJobs(Array.isArray(d?.imports)?d.imports:[])}catch{}}
 useEffect(()=>{loadJobs()},[]);
 async function previewFile(file:File){
  setBusy(true);setError("");setMessage("");setPreview(null);
  try{const fd=new FormData();fd.append("kind",kind);fd.append("send_credentials",String(sendCredentials));fd.append("file",file);const d=await api("/admin/onboarding/bulk/preview",{method:"POST",body:fd});setPreview(d);setMessage("Validation completed. Review the records before starting the import.");await loadJobs()}catch(e){setError(e instanceof Error?e.message:"Unable to validate import")}finally{setBusy(false)}
 }
 async function start(){
  if(!preview)return;setBusy(true);setError("");
  try{await api("/admin/onboarding/bulk/"+preview.job.id+"/start",{method:"POST"});setMessage("Import queued. You can leave this page; processing continues in the background.");await loadJobs();setPreview(null)}catch(e){setError(e instanceof Error?e.message:"Unable to start import")}finally{setBusy(false)}
 }
 useEffect(()=>{const running=jobs.some(j=>["queued","running"].includes(j.status));if(!running)return;const t=setInterval(loadJobs,2000);return()=>clearInterval(t)},[jobs]);
 return <div className="content standalone">
  <header className="topbar"><div><Link className="back" href="/dashboard">← Dashboard</Link><p className="eyebrow">SCHOOL ONBOARDING</p><h1>Bulk onboarding</h1><p className="muted">Import thousands of students, teachers and parents without creating accounts one by one.</p></div></header>
  {error&&<div className="error banner">{error}</div>}{message&&<div className="panel"><strong>{message}</strong></div>}
  <section className="panel">
   <div className="panel-head"><div><h2>1. Prepare your data</h2><p>Download a template, fill it in, then upload CSV or XLSX. The platform creates secure temporary passwords for new accounts.</p></div></div>
   <div className="actions">{(["students","teachers","parents"] as const).map(k=><button key={k} className={kind===k?"":"ghost"} onClick={()=>{setKind(k);setPreview(null)}}>{k[0].toUpperCase()+k.slice(1)}</button>)}</div>
   <div className="form-grid">
    <div><p><strong>{kind==="students"?"Students":kind==="teachers"?"Teachers":"Parents / Guardians"} template</strong></p><button className="ghost" onClick={()=>downloadTemplate(kind)}>Download CSV template</button></div>
    <label className="checkbox-row"><input type="checkbox" checked={sendCredentials} onChange={e=>setSendCredentials(e.target.checked)}/> Email temporary login credentials to newly created accounts</label><label>Upload file<input ref={fileRef} type="file" accept=".csv,.xlsx" onChange={e=>e.target.files?.[0]&&previewFile(e.target.files[0])}/></label>
   </div>
  </section>
  {preview&&<section className="panel">
   <div className="panel-head"><div><h2>2. Review validation</h2><p>{preview.job.total.toLocaleString()} records detected. No database import has started.</p></div><span className={"pill "+(preview.errors.length?"suspended":"active")}>{preview.errors.length?preview.errors.length+" errors":"Ready"}</span></div>
   <div className="stats"><Stat label="Total" value={preview.job.total} detail="Uploaded rows"/><Stat label="Valid" value={preview.job.valid} detail="Ready to import"/><Stat label="Errors" value={preview.errors.length} detail="Rows needing correction"/></div>
   {preview.errors.length>0&&<div className="table-wrap"><table><thead><tr><th>Row</th><th>Field</th><th>Problem</th></tr></thead><tbody>{preview.errors.slice(0,50).map((e,i)=><tr key={i}><td>{e.row}</td><td>{e.field||"—"}</td><td>{e.message}</td></tr>)}</tbody></table></div>}
   <div className="panel-head"><div><h3>Preview</h3><p>First 10 rows only.</p></div><button disabled={busy||preview.job.valid===0} onClick={start}>{busy?"Starting…":"Start import"}</button></div>
  </section>}
  <section className="panel"><div className="panel-head"><div><h2>3. Import history</h2><p>Progress and results are school-scoped.</p></div><button className="ghost" onClick={loadJobs}>Refresh</button></div>
   <div className="table-wrap"><table><thead><tr><th>Type</th><th>File</th><th>Total</th><th>Created</th><th>Skipped</th><th>Failed</th><th>Credentials</th><th>Status</th></tr></thead><tbody>{jobs.map(j=><tr key={j.id}><td>{j.kind}</td><td>{j.file_name}</td><td>{j.total.toLocaleString()}</td><td>{j.created.toLocaleString()}</td><td>{j.skipped.toLocaleString()}</td><td>{j.failed.toLocaleString()}</td><td>{j.send_credentials?`${j.credentials_sent.toLocaleString()} sent${j.credential_emails_failed?` / ${j.credential_emails_failed} failed`:""}`:"Off"}</td><td><span className={"pill "+(j.status.includes("completed")?"active":"")}>{j.status}</span></td></tr>)}</tbody></table>{!jobs.length&&<div className="empty">No bulk imports yet.</div>}</div>
  </section>
  <section className="panel"><h2>What happens next?</h2><p className="muted">Students can be enrolled into the supplied session/class/section. Parents are matched to existing students by admission number and reused when their school email matches. Teachers are created as school-scoped accounts and can then be assigned through Teacher Assignments.</p><div className="actions"><Link className="ghost" href="/dashboard/enrollments">Enrollment</Link><Link className="ghost" href="/dashboard/teacher-assignments">Teacher Assignments</Link><Link className="ghost" href="/dashboard/users">Users & Roles</Link></div></section>
 </div>
}
function Stat({label,value,detail}:{label:string;value:number;detail:string}){return <div className="panel"><span className="muted">{label}</span><h2>{value.toLocaleString()}</h2><small>{detail}</small></div>}
