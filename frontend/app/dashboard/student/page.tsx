"use client";
import {useEffect,useState} from "react";
import Link from "next/link";

type Item=Record<string,any>;
async function api(path:string){const t=localStorage.getItem("access_token");const r=await fetch("/backend"+path,{headers:{Authorization:"Bearer "+t}});const d=await r.json().catch(()=>({}));if(r.status===401)throw Error("Session expired");if(!r.ok)throw Error(d?.error?.message||"Request failed");return d}
const list=(d:any,k:string)=>Array.isArray(d)?d:d?.[k]||d?.data||[];
export default function StudentPortal(){
 const [sessionId,setSessionId]=useState(""); const [termId,setTermId]=useState("");
 const[profile,setProfile]=useState<Item|null>(null),[enrollment,setEnrollment]=useState<Item|null>(null),[terms,setTerms]=useState<Item[]>([]),[data,setData]=useState<Record<string,Item[]>>({}),[schoolName,setSchoolName]=useState(""),[error,setError]=useState(""),[portalStatus,setPortalStatus]=useState("");
 useEffect(()=>{(async()=>{try{const context=await api("/academic-context");const sessions=list(context,"sessions");const session=sessions.find((x:any)=>x.status==="active")||sessions[0];const term=session?.terms?.find((x:any)=>x.status==="active")||session?.terms?.[0];setSessionId(session?.id||"");setTermId(term?.id||"");if(!session||!term)return;
  (async()=>{
   try{
    const me=await api("/me"); setSchoolName(me?.school_name||"");
    try{setProfile(await api("/student/profile"))}catch(e:any){setError(e.message||"Unable to load student profile");return}
    let activeEnrollment:any=null;
    try{activeEnrollment=await api("/student/enrollment?academic_session_id="+sessionId);setEnrollment(activeEnrollment)}catch(e:any){
      if((e.message||"").toLowerCase().includes("student portal record not found"))setPortalStatus("No active enrollment exists for this academic session.");
      else setError(e.message||"Unable to load enrollment");
    }
    try{setTerms(list(await api("/student/terms?academic_session_id="+sessionId),"terms"))}catch(e:any){}
    if(activeEnrollment){
      const results=await Promise.allSettled([
       api("/student/attendance?academic_session_id="+sessionId+"&term_id="+termId),
       api("/student/timetable?academic_session_id="+sessionId+"&term_id="+termId),
       api("/student/invoices?academic_session_id="+sessionId+"&term_id="+termId),
       api("/student/payments?academic_session_id="+sessionId+"&term_id="+termId)
      ]);
      const value=(i:number,k:string)=>results[i].status==="fulfilled"?list((results[i] as PromiseFulfilledResult<any>).value,k):[];
      setData({attendance:value(0,"attendance"),timetable:value(1,"timetable"),invoices:value(2,"invoices"),payments:value(3,"payments")});
    }
   }catch(e:any){setError(e.message||"Unable to load student portal")}
  })().catch((e:any)=>setError(e.message||"Unable to load student portal"))
 },[]);
 return <div className="content standalone"><Link className="back" href="/dashboard">← Dashboard</Link><header className="topbar"><div><p className="eyebrow">STUDENT PORTAL</p><h1>My academic dashboard</h1><p className="muted">{schoolName||"Your school"} · Your school records, attendance, timetable and report cards.</p></div></header>
 {error&&<div className="error banner">{error}</div>}
 {portalStatus&&<div className="panel"><strong>Enrollment status</strong><p className="muted">{portalStatus} Ask your school administrator to enroll you in the current academic session.</p></div>}
 {profile&&<section className="panel"><div className="panel-head"><div><h2>{profile.name}</h2><p>{profile.admission_number} · {profile.email}</p></div><span className="badge">{profile.enrollment_status}</span></div>{enrollment&&<div className="module-grid"><div className="module"><div className="module-icon">C</div><div><strong>{enrollment.class_name}</strong><p>{enrollment.section_name||"No section"} · {enrollment.session_name}</p></div></div></div>}</section>}
 <section className="stats"><Stat label="Attendance" value={data.attendance?.length||0}/><Stat label="Invoices" value={data.invoices?.length||0}/><Stat label="Payments" value={data.payments?.length||0}/><Stat label="Terms" value={terms.length}/></section>
 <section className="grid-2"><Panel title="Attendance" text="Your attendance records."><Table rows={data.attendance||[]} cols={["date","status","remarks"]}/></Panel><Panel title="Timetable" text="Your current class timetable."><Table rows={data.timetable||[]} cols={["day_of_week","start_time","end_time","room"]}/></Panel></section>
 <section className="grid-2"><Panel title="Invoices" text="Your school billing records."><Table rows={data.invoices||[]} cols={["invoice_number","total_amount","paid_amount","balance","status"]}/></Panel><Panel title="Payments" text="Your recorded payments."><Table rows={data.payments||[]} cols={["amount","provider","reference","status"]}/></Panel></section>
 <section className="panel"><div className="panel-head"><div><h2>Report cards</h2><p>Open results for a term in your current academic session.</p></div></div><div className="module-grid">{terms.map(t=><Link className="module" key={t.id} href={"/dashboard/student/report-card?termId="+t.id+"&sessionId="+sessionId}><div className="module-icon">R</div><div><strong>{t.name}</strong><p>{t.start_date} to {t.end_date}</p></div><span>Open</span></Link>)}</div>{!terms.length&&<div className="empty">No academic terms available.</div>}</section>
 </div>
}
function Stat({label,value}:{label:string;value:number}){return <div className="stat"><span>{label}</span><strong>{value}</strong></div>}
function Panel(p:{title:string;text:string;children:React.ReactNode}){return <section className="panel"><div className="panel-head"><div><h2>{p.title}</h2><p>{p.text}</p></div></div>{p.children}</section>}
function Table({rows,cols}:{rows:Item[];cols:string[]}){return <div className="table-wrap"><table><thead><tr>{cols.map(c=><th key={c}>{c}</th>)}</tr></thead><tbody>{rows.map((x,i)=><tr key={x.id||i}>{cols.map(c=><td key={c}>{String(x[c]??"—")}</td>)}</tr>)}</tbody></table>{!rows.length&&<div className="empty">No records.</div>}</div>}
