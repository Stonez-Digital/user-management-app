"use client";

import {useEffect,useMemo,useState} from "react";
import Link from "next/link";
import {useRouter} from "next/navigation";


type User={id:string;name:string;email:string;role:string;active:boolean;school_name?:string|null};
type Assignment={id:string;teacher_id:string;subject_id:string;academic_session_id:string;term_id:string;class_id:string;section_id?:string|null;active:boolean;subject?:{name:string;code?:string}};
type Timetable={id:string;teacher_assignment_id:string;class_id:string;section_id?:string|null;day_of_week:number;start_time:string;end_time:string;room?:string;active:boolean};
type Session={id:string;name:string;status:string;start_date:string;end_date:string};
type Term={id:string;name:string;status:string;academic_session_id:string};

async function api(path:string){
  const token=localStorage.getItem("access_token");
  const r=await fetch("/backend"+path,{headers:{Authorization:"Bearer "+token}});
  const text=await r.text(); let d:any={}; try{d=text?JSON.parse(text):{}}catch{d={error:text}}
  if(r.status===401)throw Error("Session expired");
  if(!r.ok)throw Error(d?.error?.message||d?.error||"Request failed");
  return d;
}
const list=(d:any,k:string)=>Array.isArray(d)?d:d?.[k]||[];
const day=(n:number)=>["","Monday","Tuesday","Wednesday","Thursday","Friday","Saturday"][n]||"";
export default function TeacherDashboard(){
  const [selectedSessionId,setSelectedSessionId]=useState(""); const [selectedTermId,setSelectedTermId]=useState("");
  const router=useRouter(); const [me,setMe]=useState<User|null>(null); const [assignments,setAssignments]=useState<Assignment[]>([]);
  const [timetable,setTimetable]=useState<Timetable[]>([]); const [sessions,setSessions]=useState<Session[]>([]); const [terms,setTerms]=useState<Term[]>([]); const [notifications,setNotifications]=useState<any[]>([]);
  const [error,setError]=useState("");
  useEffect(()=>{(async()=>{try{
    const context=await api("/academic-context"); const ss=list(context,"sessions"); const active=ss.find((x:Session)=>x.status==="active")||ss[0]; const selected=active?.id||""; const activeTerm=active?.terms?.find((x:Term)=>x.status==="active")||active?.terms?.[0]; setSelectedSessionId(selected); setSelectedTermId(activeTerm?.id||""); if(!selected||!activeTerm)return;
    const m=await api("/me"); if(m?.role!=="teacher"){router.replace("/dashboard");return} setMe(m);
    const [a,t,s,n]=await Promise.all([api("/teacher/assignments?academic_session_id="+selectedSessionId+"&term_id="+selectedTermId),api("/teacher/timetable?academic_session_id="+selectedSessionId+"&term_id="+selectedTermId),api("/admin/academic-sessions"),api("/notifications")]);
    setAssignments(list(a,"assignments")); setTimetable(list(t,"timetable")); setNotifications(list(n,"notifications")); const ss=list(s,"sessions"); setSessions(ss);
    const current=ss.find((x:Session)=>x.id===selected); if(current){setTerms(current.terms||[])}
  }catch(e){const msg=e instanceof Error?e.message:"Unable to load teacher workspace";setError(msg);if(msg==="Session expired")router.replace("/")}})()},[router,selectedSessionId,selectedTermId]);
  const today=new Date().getDay()||7;
  const todaySlots=useMemo(()=>timetable.filter(x=>x.active&&x.day_of_week===today).sort((a,b)=>a.start_time.localeCompare(b.start_time)),[timetable,today]);
  const activeSession=sessions.find(x=>x.status==="active")||sessions[0];
  const activeTerm=terms.find(x=>x.status==="active")||terms[0];
  const logout=()=>{localStorage.clear();router.replace("/")};
  return <div className="app-shell"><aside className="sidebar"><div className="logo"><span>S</span><div><strong>Stonez</strong><small>School OS</small></div></div>
    <nav><Link className="active" href="/dashboard/teacher">My Workspace</Link><Link href="/dashboard/notifications">Notifications ({notifications.filter(x=>!x.read).length})</Link><Link href="/dashboard/teacher/attendance">Attendance</Link><Link href="/dashboard/teacher/assessments">Assessments</Link><Link href="/dashboard/teacher/results">Results</Link></nav>
    <div className="sidebar-bottom"><div className="mini-user"><div className="avatar">{me?.name?.[0]||"T"}</div><div><strong>{me?.name||"Teacher"}</strong><small>Teacher</small></div></div><button className="ghost" onClick={logout}>Sign out</button></div>
  </aside><main className="content"><header className="topbar"><div><p className="eyebrow">TEACHER WORKSPACE</p><h1>Welcome, {me?.name||"Teacher"}</h1><p className="muted">{me?.school_name||"Your school"} · Your classes, subjects and teaching schedule in one place.</p></div><div className="status"><span/> {me?.school_name||"School workspace"}</div></header>
  {error&&<div className="error banner">{error}</div>}
  <section className="stats"><div className="stat"><span>Assignments</span><strong>{assignments.filter(x=>x.active).length}</strong><small>Active teaching assignments</small></div><div className="stat"><span>Subjects</span><strong>{new Set(assignments.filter(x=>x.active).map(x=>x.subject_id)).size}</strong><small>Subjects assigned</small></div><div className="stat"><span>Classes</span><strong>{new Set(assignments.filter(x=>x.active).map(x=>x.class_id)).size}</strong><small>Classes assigned</small></div><div className="stat"><span>Term</span><strong>{activeTerm?.name||"—"}</strong><small>{activeSession?.name||"No active session"}</small></div></section>
  <section className="grid-2"><div className="panel"><div className="panel-head"><div><h2>My assignments</h2><p>Only assignments belonging to your teacher account.</p></div></div><div className="table-wrap"><table><thead><tr><th>Subject</th><th>Class</th><th>Section</th><th>Term</th></tr></thead><tbody>{assignments.filter(x=>x.active).map(a=><tr key={a.id}><td><strong>{a.subject?.code||"Subject"}</strong><small>{a.subject?.name||a.subject_id}</small></td><td>{(a as any).class?.name||a.class_id}</td><td>{(a as any).section?.name||"Whole class"}</td><td>{a.term_id}</td></tr>)}</tbody></table>{!assignments.length&&<div className="empty">No teaching assignments yet.</div>}</div></div>
  <div className="panel"><div className="panel-head"><div><h2>Today's timetable</h2><p>{day(today)}</p></div><Link href="/dashboard/teacher">View workspace →</Link></div><div className="role-list">{todaySlots.map(x=><div key={x.id}><span>{x.start_time}–{x.end_time} · {x.room||"No room"}</span><strong>{x.class_id}</strong></div>)}</div>{!todaySlots.length&&<div className="empty">No classes scheduled today.</div>}</div></section>
  <section className="panel"><div className="panel-head"><div><h2>Notifications</h2><p>School announcements and operational alerts.</p></div><Link href="/dashboard/notifications">Open notification center →</Link></div>{notifications.slice(0,5).map(n=><article className="module" key={n.id}><div className="module-icon">!</div><div><strong>{n.title}</strong><p>{n.body}</p><small>{n.category} · {n.read?"Read":"Unread"}</small></div></article>)}{!notifications.length&&<div className="empty">No notifications.</div>}</section>
  <section className="panel"><div className="panel-head"><div><h2>Teacher tools</h2><p>Operational workflows will stay limited to your assigned classes and subjects.</p></div></div><div className="module-grid"><Link className="module" href="/dashboard/attendance"><div className="module-icon">A</div><div><strong>Attendance</strong><p>Record and review attendance for assigned classes.</p></div><span>Open</span></Link><Link className="module" href="/dashboard/assessments"><div className="module-icon">C</div><div><strong>Assessments</strong><p>Create and manage assessments for your assignments.</p></div><span>Open</span></Link><Link className="module" href="/dashboard/results"><div className="module-icon">R</div><div><strong>Results</strong><p>Enter and review student scores.</p></div><span>Open</span></Link></div></section>
  </main></div>
}