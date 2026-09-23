"use client";

import {useEffect,useMemo,useState} from "react";
import Link from "next/link";
import { useAcademicContext } from "../../../lib/academic-context";

type Enrollment={id:string;status:string;student?:{admission_number:string;user?:{name:string}};academic_session?:{id:string;name:string};class?:{name:string};section?:{name:string}};
type Term={id:string;name:string;status:string;start_date:string;end_date:string};
type Attendance={id:string;date:string;status:string;note?:string;enrollment_id?:string;term_id?:string;enrollment?:Enrollment;term?:Term};

async function api(path:string,options:RequestInit={}) {
  const token=localStorage.getItem("access_token");
  const r=await fetch("/backend"+path,{...options,headers:{...options.headers,Authorization:"Bearer "+token,"Content-Type":"application/json"}});
  const text=await r.text();let d:any={};try{d=text?JSON.parse(text):{}}catch{d={error:text}};
  if(r.status===401)throw Error("Session expired");
  if(!r.ok)throw Error(d?.error?.message||d?.error||"Request failed");
  return d;
}

export default function AttendancePage(){
  const { sessionId: selectedSessionId, termId: selectedTermId, terms: selectedTerms } = useAcademicContext();
  const [enrollments,setEnrollments]=useState<Enrollment[]>([]);
  const [terms,setTerms]=useState<Term[]>([]);
  const [items,setItems]=useState<Attendance[]>([]);
  const [enrollmentId,setEnrollmentId]=useState("");
  const [termId,setTermId]=useState("");
  const [date,setDate]=useState(new Date().toISOString().slice(0,10));
  const [status,setStatus]=useState("present");
  const [note,setNote]=useState("");
  const [query,setQuery]=useState("");
  const [error,setError]=useState("");
  const [busy,setBusy]=useState(false);

  async function load(){
    try{
      const [e,a]=await Promise.all([api("/admin/enrollments"),api("/admin/attendance")]);
      setEnrollments((Array.isArray(e)?e:e?.enrollments||[]).filter((x:Enrollment)=>!selectedSessionId||x.academic_session?.id===selectedSessionId));
      setItems(Array.isArray(a)?a:a?.attendance||[]);
    }catch(err){setError(err instanceof Error?err.message:"Unable to load attendance data")}
  }
  useEffect(()=>{load()},[]);

  const selectedEnrollment=useMemo(()=>enrollments.find(e=>e.id===enrollmentId),[enrollments,enrollmentId]);
  useEffect(()=>{
    const session = selectedEnrollment?.academic_session?.id;
    if(!session || (selectedSessionId && session !== selectedSessionId)){setTerms([]);setTermId("");return}
    setTerms(selectedTerms as Term[]);
    setTermId(selectedTermId && selectedTerms.some(t=>t.id===selectedTermId) ? selectedTermId : "");
  },[selectedEnrollment,selectedSessionId,selectedTermId,selectedTerms]);

  async function create(){
    if(!enrollmentId||!termId||!date){setError("Select an enrollment, term and date.");return}
    setBusy(true);setError("");
    try{
      await api("/admin/attendance",{method:"POST",body:JSON.stringify({enrollment_id:enrollmentId,term_id:termId,date,status,note})});
      setEnrollmentId("");setTermId("");setNote("");setStatus("present");await load();
    }catch(err){setError(err instanceof Error?err.message:"Unable to record attendance")}finally{setBusy(false)}
  }

  async function update(id:string,next:string){
    setError("");
    const current=items.find(x=>x.id===id);if(!current)return;
    try{
      await api("/admin/attendance/"+id,{method:"PUT",body:JSON.stringify({enrollment_id:current.enrollment_id||current.enrollment?.id,term_id:current.term_id||current.term?.id,date:current.date,status:next,note:current.note||""})});
      await load();
    }catch(err){setError(err instanceof Error?err.message:"Unable to update attendance")}
  }

  const filtered=items.filter(a=>(a.enrollment?.student?.user?.name+" "+a.enrollment?.student?.admission_number+" "+a.enrollment?.class?.name+" "+a.enrollment?.section?.name+" "+a.status).toLowerCase().includes(query.toLowerCase()));

  return <div className="content standalone">
    <header className="topbar"><div><Link className="back" href="/dashboard">← Dashboard</Link><p className="eyebrow">ACADEMIC OPERATIONS</p><h1>Attendance</h1><p className="muted">Record one attendance status per enrolled student per day.</p></div></header>
    {error&&<div className="error banner">{error}</div>}
    <section className="panel"><div className="panel-head"><div><h2>Record attendance</h2><p>Attendance is tied to the student's enrollment and academic term.</p></div></div>
      <div className="form-grid">
        <label>Enrollment<select value={enrollmentId} onChange={e=>setEnrollmentId(e.target.value)}><option value="">Select enrolled student</option>{enrollments.filter(e=>e.status==="active").map(e=><option key={e.id} value={e.id}>{e.student?.user?.name||"Student"} — {e.student?.admission_number} · {e.class?.name||""} {e.section?.name||""}</option>)}</select></label>
        <label>Term<select value={termId} onChange={e=>setTermId(e.target.value)} disabled={!enrollmentId}><option value="">Select term</option>{terms.map(t=><option key={t.id} value={t.id}>{t.name} ({t.status})</option>)}</select></label>
        <label>Date<input type="date" value={date} onChange={e=>setDate(e.target.value)}/></label>
        <label>Status<select value={status} onChange={e=>setStatus(e.target.value)}><option value="present">Present</option><option value="absent">Absent</option><option value="late">Late</option><option value="excused">Excused</option></select></label>
        <label>Note<input value={note} maxLength={500} onChange={e=>setNote(e.target.value)} placeholder="Optional note"/></label>
        <div className="form-action"><button disabled={busy} onClick={create}>{busy?"Saving...":"Record attendance"}</button></div>
      </div>
    </section>
    <section className="panel"><div className="panel-head"><div><h2>Attendance records</h2><p>{filtered.length} record{filtered.length===1?"":"s"}</p></div><input className="search-inline" placeholder="Search..." value={query} onChange={e=>setQuery(e.target.value)}/></div>
      <div className="table-wrap"><table><thead><tr><th>Date</th><th>Student</th><th>Class</th><th>Term</th><th>Status</th><th>Action</th></tr></thead><tbody>{filtered.map(a=><tr key={a.id}><td>{a.date?.slice(0,10)||"—"}</td><td><strong>{a.enrollment?.student?.user?.name||"Student"}</strong><small>{a.enrollment?.student?.admission_number||""}</small></td><td>{a.enrollment?.class?.name||"—"} {a.enrollment?.section?.name||""}</td><td>{a.term?.name||"—"}</td><td><span className={"pill "+a.status}>{a.status}</span></td><td><select value={a.status} onChange={e=>update(a.id,e.target.value)}><option value="present">present</option><option value="absent">absent</option><option value="late">late</option><option value="excused">excused</option></select></td></tr>)}</tbody></table>{!filtered.length&&<div className="empty">No attendance records found.</div>}</div>
    </section>
  </div>
}
