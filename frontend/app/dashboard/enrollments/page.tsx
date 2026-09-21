"use client";
import {useEffect,useMemo,useState} from "react";
import Link from "next/link";

type Student={id:string;admission_number:string;user?:{name:string;email:string}};
type Session={id:string;name:string;status:string};
type SchoolClass={id:string;name:string;level:number;sections?:Section[]};
type Section={id:string;class_id:string;name:string};
type Enrollment={id:string;status:string;student?:Student;academic_session?:Session;class?:SchoolClass;section?:Section};

async function api(path:string, options:RequestInit={}) {
  const token=localStorage.getItem("access_token");
  const r=await fetch("/backend"+path,{...options,headers:{...options.headers,Authorization:"Bearer "+token,"Content-Type":"application/json"}});
  const text=await r.text(); let d:any={}; try{d=text?JSON.parse(text):{}}catch{d={error:text}}
  if(r.status===401) throw Error("Session expired");
  if(!r.ok) throw Error(d?.error?.message||d?.error||"Request failed");
  return d;
}

export default function EnrollmentPage(){
  const [students,setStudents]=useState<Student[]>([]),[sessions,setSessions]=useState<Session[]>([]),[classes,setClasses]=useState<SchoolClass[]>([]);
  const [items,setItems]=useState<Enrollment[]>([]),[studentId,setStudentId]=useState(""),[sessionId,setSessionId]=useState(""),[classId,setClassId]=useState(""),[sectionId,setSectionId]=useState(""),[status,setStatus]=useState("active");
  const [historyStudent,setHistoryStudent]=useState<Student|null>(null),[history,setHistory]=useState<Enrollment[]>([]),[historyBusy,setHistoryBusy]=useState(false);
  const [query,setQuery]=useState(""),[error,setError]=useState(""),[busy,setBusy]=useState(false);
  const sections=useMemo(()=>classes.find(c=>c.id===classId)?.sections||[],[classes]);
  async function load(){
    try{const [s,a,c,e]=await Promise.all([api("/admin/students"),api("/admin/academic-sessions"),api("/admin/classes"),api("/admin/enrollments")]);
      setStudents(Array.isArray(s)?s:s?.students||[]);setSessions(Array.isArray(a)?a:a?.sessions||[]);setClasses(Array.isArray(c)?c:c?.classes||[]);setItems(Array.isArray(e)?e:e?.enrollments||[]);
    }catch(err){setError(err instanceof Error?err.message:"Unable to load enrollment data")}
  }
  useEffect(()=>{load()},[]);
  async function create(){
    if(!studentId||!sessionId||!classId||!sectionId){setError("Select a student, academic session, class and section.");return}
    setBusy(true);setError("");
    try{await api("/admin/enrollments",{method:"POST",body:JSON.stringify({student_id:studentId,academic_session_id:sessionId,class_id:classId,section_id:sectionId,status})});setStudentId("");setSessionId("");setClassId("");setSectionId("");setStatus("active");await load()}
    catch(err){setError(err instanceof Error?err.message:"Unable to create enrollment")}finally{setBusy(false)}
  }
  async function update(id:string,next:string){setError("");try{await api("/admin/enrollments/"+id,{method:"PUT",body:JSON.stringify({status:next})});await load()}catch(err){setError(err instanceof Error?err.message:"Unable to update enrollment")}}
  async function showHistory(student:Student){setHistoryStudent(student);setHistoryBusy(true);setError("");try{const d=await api("/admin/enrollments/student/"+student.id);setHistory(Array.isArray(d)?d:d?.enrollments||[])}catch(err){setError(err instanceof Error?err.message:"Unable to load enrollment history")}finally{setHistoryBusy(false)}}
  const filtered=items.filter(e=>(e.student?.user?.name+" "+e.student?.admission_number+" "+e.academic_session?.name+" "+e.class?.name+" "+e.section?.name).toLowerCase().includes(query.toLowerCase()));
  return <div className="content standalone">
    <header className="topbar"><div><Link className="back" href="/dashboard">← Dashboard</Link><p className="eyebrow">ACADEMIC OPERATIONS</p><h1>Student Enrollment</h1><p className="muted">Place students into a class and section for an academic session.</p></div></header>
    {error&&<div className="error banner">{error}</div>}
    <section className="panel"><div className="panel-head"><div><h2>Enroll a student</h2><p>Each student can have one enrollment per academic session.</p></div></div>
      <div className="form-grid">
        <label>Student<select value={studentId} onChange={e=>setStudentId(e.target.value)}><option value="">Select student</option>{students.map(s=><option key={s.id} value={s.id}>{s.user?.name||"Student"} — {s.admission_number}</option>)}</select></label>
        <label>Academic session<select value={sessionId} onChange={e=>setSessionId(e.target.value)}><option value="">Select session</option>{sessions.map(s=><option key={s.id} value={s.id}>{s.name} ({s.status})</option>)}</select></label>
        <label>Class<select value={classId} onChange={e=>{setClassId(e.target.value);setSectionId("")}}><option value="">Select class</option>{classes.map(c=><option key={c.id} value={c.id}>{c.name}</option>)}</select></label>
        <label>Section<select value={sectionId} onChange={e=>setSectionId(e.target.value)} disabled={!classId}><option value="">Select section</option>{sections.map(s=><option key={s.id} value={s.id}>{s.name}</option>)}</select></label>
        <label>Status<select value={status} onChange={e=>setStatus(e.target.value)}><option value="active">Active</option><option value="completed">Completed</option><option value="withdrawn">Withdrawn</option></select></label>
        <div className="form-action"><button disabled={busy} onClick={create}>{busy?"Enrolling...":"Enroll student"}</button></div>
      </div>
    </section>
    <section className="panel"><div className="panel-head"><div><h2>Current enrollments</h2><p>{filtered.length} enrollment record{filtered.length===1?"":"s"}</p></div><input className="search-inline" placeholder="Search..." value={query} onChange={e=>setQuery(e.target.value)}/></div>
      <div className="table-wrap"><table><thead><tr><th>Student</th><th>Session</th><th>Class</th><th>Section</th><th>Status</th><th>Action</th><th>History</th></tr></thead><tbody>{filtered.map(e=><tr key={e.id}><td><strong>{e.student?.user?.name||"Student"}</strong><small>{e.student?.admission_number}</small></td><td>{e.academic_session?.name||"—"}</td><td>{e.class?.name||"—"}</td><td>{e.section?.name||"—"}</td><td><span className={"pill "+e.status}>{e.status}</span></td><td><select value={e.status} onChange={x=>update(e.id,x.target.value)}><option value="active">active</option><option value="completed">completed</option><option value="withdrawn">withdrawn</option></select></td><td><button className="ghost" onClick={()=>e.student&&showHistory(e.student)}>View history</button></td></tr>)}</tbody></table>{!filtered.length&&<div className="empty">No enrollments found.</div>}</div>
    </section>
    {historyStudent&&<section className="panel"><div className="panel-head"><div><h2>Enrollment history</h2><p>{historyStudent.user?.name||"Student"} · {historyStudent.admission_number}</p></div><button className="ghost" onClick={()=>setHistoryStudent(null)}>Close</button></div>{historyBusy?<div className="empty">Loading history...</div>:<div className="table-wrap"><table><thead><tr><th>Session</th><th>Class</th><th>Section</th><th>Status</th><th>Enrolled</th></tr></thead><tbody>{history.map(h=><tr key={h.id}><td>{h.academic_session?.name||"—"}</td><td>{h.class?.name||"—"}</td><td>{h.section?.name||"—"}</td><td><span className={"pill "+h.status}>{h.status}</span></td><td>{h.enrolled_at?.slice(0,10)||"—"}</td></tr>)}</tbody></table>{!history.length&&<div className="empty">No enrollment history found.</div>}</div>}</section>}
  </div>
}