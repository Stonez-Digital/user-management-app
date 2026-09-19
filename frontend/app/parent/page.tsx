"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";

type Child = { student_id:string; name:string; admission_number:string; relationship:string; primary:boolean };
type Attendance = { date:string; status:string; note?:string };
type Invoice = { id:string; invoice_number:string; total_amount:number; paid_amount:number; balance:number; status:string; due_date?:string };
type Payment = { id:string; invoice_id:string; amount:number; provider:string; reference:string; status:string; paid_at?:string };
type Timetable = { day_of_week:number; start_time:string; end_time:string; room?:string };
type Report = { items?:Array<{subject_name:string; score:number; max_score:number; percentage:number}> };

async function api(path:string){
  const token=localStorage.getItem("access_token");
  const r=await fetch("/backend"+path,{headers:{Authorization:"Bearer "+token}});
  const text=await r.text(); let d:any={}; try{d=text?JSON.parse(text):{}}catch{d={error:text}};
  if(r.status===401) throw Error("Session expired");
  if(!r.ok) throw Error(d?.error?.message||d?.error||"Request failed");
  return d;
}

const days=["","Monday","Tuesday","Wednesday","Thursday","Friday","Saturday"];

export default function ParentPortal(){
  const router=useRouter();
  const [children,setChildren]=useState<Child[]>([]);
  const [selected,setSelected]=useState("");
  const [attendance,setAttendance]=useState<Attendance[]>([]);
  const [invoices,setInvoices]=useState<Invoice[]>([]);
  const [payments,setPayments]=useState<Payment[]>([]);
  const [timetable,setTimetable]=useState<Timetable[]>([]);
  const [report,setReport]=useState<Report>({});
  const [error,setError]=useState("");
  const [loading,setLoading]=useState(true);

  useEffect(()=>{api("/parent/children").then((d)=>{const list=Array.isArray(d)?d:d?.children||[];setChildren(list);if(list[0])setSelected(list[0].student_id)}).catch(e=>{setError(e.message);if(e.message==="Session expired")router.push("/")}).finally(()=>setLoading(false))},[router]);

  useEffect(()=>{
    if(!selected)return;
    setError("");
    Promise.all([
      api("/parent/children/"+selected+"/attendance"),
      api("/parent/children/"+selected+"/invoices"),
      api("/parent/children/"+selected+"/payments"),
      api("/parent/children/"+selected+"/timetable"),
    ]).then(([a,i,p,t])=>{setAttendance(Array.isArray(a)?a:a?.attendance||[]);setInvoices(Array.isArray(i)?i:i?.invoices||[]);setPayments(Array.isArray(p)?p:p?.payments||[]);setTimetable(Array.isArray(t)?t:t?.timetable||[])})
      .catch(e=>setError(e.message));
  },[selected]);

  const outstanding=useMemo(()=>invoices.reduce((sum,i)=>sum+Number(i.balance||0),0),[invoices]);
  const paid=useMemo(()=>payments.filter(p=>p.status==="successful").reduce((sum,p)=>sum+Number(p.amount||0),0),[payments]);

  async function loadReport(termId:string){
    try{const d=await api("/parent/children/"+selected+"/report-cards/"+termId);setReport(d)}catch(e){setError(e instanceof Error?e.message:"Unable to load report card")}
  }

  if(loading)return <main className="content standalone"><div className="panel"><h2>Loading parent portal...</h2></div></main>;

  return <div className="app-shell">
    <aside className="sidebar">
      <div className="logo"><span>S</span><div><strong>Stonez</strong><small>Parent Portal</small></div></div>
      <nav><Link className="active" href="/parent">Overview</Link><Link href="/parent">Children</Link></nav>
      <div className="sidebar-bottom"><button className="ghost" onClick={()=>{localStorage.clear();router.push("/")}}>Sign out</button></div>
    </aside>
    <main className="content">
      <header className="topbar"><div><p className="eyebrow">FAMILY PORTAL</p><h1>Parent dashboard</h1><p className="muted">Stay connected to your child’s school progress.</p></div></header>
      {error&&<div className="error banner">{error}</div>}
      <section className="panel">
        <div className="panel-head"><div><h2>Your children</h2><p>Select a child to view their school information.</p></div></div>
        <div className="module-grid">{children.map(c=><button key={c.student_id} className={"module "+(selected===c.student_id?"selected":"")} onClick={()=>setSelected(c.student_id)}><div className="module-icon">{c.name?.[0]||"S"}</div><div><strong>{c.name}</strong><p>{c.admission_number} · {c.relationship}</p></div><span>{c.primary?"Primary":"Linked"}</span></button>)}</div>
        {!children.length&&<div className="empty">No children are linked to this parent account yet. Please contact the school administrator.</div>}
      </section>
      {selected&&<><section className="stats"><div className="stat"><span>Outstanding fees</span><strong>₦{outstanding.toLocaleString()}</strong><small>Current invoice balance</small></div><div className="stat"><span>Payments</span><strong>₦{paid.toLocaleString()}</strong><small>Successful payments</small></div><div className="stat"><span>Attendance</span><strong>{attendance.length}</strong><small>Recorded attendance entries</small></div><div className="stat"><span>Classes</span><strong>{timetable.length}</strong><small>Timetable periods</small></div></section>
      <section className="grid-2">
        <div className="panel"><div className="panel-head"><div><h2>Recent attendance</h2><p>Latest attendance records</p></div></div><div className="table-wrap"><table><thead><tr><th>Date</th><th>Status</th></tr></thead><tbody>{attendance.slice(0,8).map((x,i)=><tr key={i}><td>{x.date}</td><td><span className={"pill "+x.status}>{x.status}</span></td></tr>)}</tbody></table>{!attendance.length&&<div className="empty">No attendance records.</div>}</div></div>
        <div className="panel"><div className="panel-head"><div><h2>Fees</h2><p>Invoices and balances</p></div></div><div className="table-wrap"><table><thead><tr><th>Invoice</th><th>Status</th><th>Balance</th></tr></thead><tbody>{invoices.slice(0,8).map(x=><tr key={x.id}><td>{x.invoice_number}</td><td><span className={"pill "+x.status}>{x.status}</span></td><td>₦{Number(x.balance||0).toLocaleString()}</td></tr>)}</tbody></table>{!invoices.length&&<div className="empty">No invoices.</div>}</div></div>
      </section>
      <section className="panel"><div className="panel-head"><div><h2>Timetable</h2><p>Current class schedule</p></div></div><div className="table-wrap"><table><thead><tr><th>Day</th><th>Time</th><th>Room</th></tr></thead><tbody>{timetable.map((x,i)=><tr key={i}><td>{days[x.day_of_week]||"Day "+x.day_of_week}</td><td>{x.start_time} – {x.end_time}</td><td>{x.room||"—"}</td></tr>)}</tbody></table>{!timetable.length&&<div className="empty">No timetable entries.</div>}</div></section>
      <section className="panel"><div className="panel-head"><div><h2>Report card</h2><p>Load a term report card using its term ID.</p></div></div><div className="form-row"><input id="term-id" placeholder="Term ID"/><button onClick={()=>{const v=(document.getElementById("term-id") as HTMLInputElement)?.value.trim();if(v)loadReport(v)}}>Load report</button></div>{report.items?.length?<div className="table-wrap"><table><thead><tr><th>Subject</th><th>Score</th><th>Percentage</th></tr></thead><tbody>{report.items.map((x,i)=><tr key={i}><td>{x.subject_name}</td><td>{x.score} / {x.max_score}</td><td>{Number(x.percentage||0).toFixed(1)}%</td></tr>)}</tbody></table></div>:null}</section></>}
    </main>
  </div>;
}
