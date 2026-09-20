"use client";
import {useEffect,useMemo,useState} from "react";
import Link from "next/link";

async function api(path:string,o:RequestInit={}) {
  const t=localStorage.getItem("access_token");
  const r=await fetch("/backend"+path,{...o,headers:{...o.headers,Authorization:"Bearer "+t,"Content-Type":"application/json"}});
  const d=await r.json().catch(()=>({}));
  if(r.status===401)throw Error("Session expired");
  if(!r.ok)throw Error(d?.error?.message||d?.error||"Request failed");
  return d;
}
const arr=(d:any,k:string)=>Array.isArray(d)?d:d?.[k]||d?.data||[];

export default function Results(){
  const [results,setResults]=useState<any[]>([]),[enrollments,setEnrollments]=useState<any[]>([]),[assessments,setAssessments]=useState<any[]>([]);
  const [eid,setEid]=useState(""),[aid,setAid]=useState(""),[score,setScore]=useState(""),[termId,setTermId]=useState("");
  const [terms,setTerms]=useState<any[]>([]),[report,setReport]=useState<any|null>(null),[error,setError]=useState(""),[busy,setBusy]=useState(false);

  async function load(){
    try{
      const [r,e,a]=await Promise.all([api("/admin/assessment-results"),api("/admin/enrollments"),api("/admin/assessments")]);
      setResults(arr(r,"results"));setEnrollments(arr(e,"enrollments"));setAssessments(arr(a,"assessments"));setError("");
    }catch(x:any){setError(x.message)}
  }
  useEffect(()=>{load()},[]);

  const selectedEnrollment=useMemo(()=>enrollments.find(x=>x.id===eid),[enrollments,eid]);
  useEffect(()=>{
    setReport(null);
    const sessionId=selectedEnrollment?.academic_session_id||selectedEnrollment?.academic_session?.id;
    if(!sessionId){setTerms([]);setTermId("");return}
    api("/admin/academic-sessions/"+sessionId+"/terms").then(d=>{
      const next=arr(d,"terms");setTerms(next);setTermId(next.find((x:any)=>x.status==="active")?.id||next[0]?.id||"");
    }).catch((x:any)=>setError(x.message));
  },[selectedEnrollment]);

  async function create(){
    setBusy(true);setError("");
    try{
      await api("/admin/assessment-results",{method:"POST",body:JSON.stringify({assessment_id:aid,student_enrollment_id:eid,score:Number(score)})});
      setScore("");await load();
    }catch(x:any){setError(x.message)}finally{setBusy(false)}
  }
  async function loadReport(){
    if(!eid||!termId){setError("Select an enrollment and term.");return}
    try{setReport(await api("/admin/report-cards/"+eid+"?term_id="+encodeURIComponent(termId)));setError("")}
    catch(x:any){setError(x.message)}
  }

  return <div className="content standalone">
    <Link className="back" href="/dashboard">← Dashboard</Link>
    <header className="topbar"><div><p className="eyebrow">ACADEMIC RESULTS</p><h1>Results & Report Cards</h1><p className="muted">Capture assessment scores and generate a term report card from verified results.</p></div></header>
    {error&&<div className="error banner">{error}</div>}

    <section className="panel"><div className="panel-head"><div><h2>Record result</h2><p>Scores are validated against the assessment and the student's school enrollment.</p></div></div>
      <div className="form-grid">
        <label>Enrollment<select value={eid} onChange={x=>setEid(x.target.value)}><option value="">Select enrollment</option>{enrollments.filter(x=>x.status!=="withdrawn"&&x.status!=="completed").map(x=><option key={x.id} value={x.id}>{x.student?.user?.name||x.student_id} · {x.class?.name||""} {x.section?.name||""}</option>)}</select></label>
        <label>Assessment<select value={aid} onChange={x=>setAid(x.target.value)}><option value="">Select assessment</option>{assessments.map(x=><option key={x.id} value={x.id}>{x.title} / {x.max_score}</option>)}</select></label>
        <label>Score<input type="number" value={score} onChange={x=>setScore(x.target.value)} min="0"/></label>
        <div className="form-action"><button disabled={!eid||!aid||score===""||busy} onClick={create}>{busy?"Saving...":"Save result"}</button></div>
      </div>
    </section>

    <section className="panel"><div className="panel-head"><div><h2>Term report card</h2><p>Generate a school-scoped report from the student's assessment results.</p></div></div>
      <div className="form-grid">
        <label>Student enrollment<select value={eid} onChange={x=>setEid(x.target.value)}><option value="">Select enrollment</option>{enrollments.map(x=><option key={x.id} value={x.id}>{x.student?.user?.name||x.student_id} · {x.class?.name||""}</option>)}</select></label>
        <label>Term<select value={termId} onChange={x=>setTermId(x.target.value)} disabled={!eid}><option value="">Select term</option>{terms.map(x=><option key={x.id} value={x.id}>{x.name} ({x.status})</option>)}</select></label>
        <div className="form-action"><button disabled={!eid||!termId} onClick={loadReport}>Generate report card</button></div>
      </div>
      {report&&<div className="report-card">
        <div className="report-summary"><div><span>Overall percentage</span><strong>{report.overall_percentage?.toFixed?.(2)??"0.00"}%</strong></div><div><span>Weighted contribution</span><strong>{report.total_weighted_contribution?.toFixed?.(2)??"0.00"}</strong></div></div>
        <div className="table-wrap"><table><thead><tr><th>Subject</th><th>Assessments</th><th>Score</th><th>Maximum</th><th>Percentage</th><th>Weighted</th></tr></thead><tbody>{(report.subjects||[]).map((s:any)=><tr key={s.subject_id}><td><strong>{s.subject_name}</strong></td><td>{s.assessment_count}</td><td>{s.total_score}</td><td>{s.total_max_score}</td><td>{Number(s.percentage||0).toFixed(2)}%</td><td>{Number(s.weighted_contribution||0).toFixed(2)}</td></tr>)}</tbody></table>{!report.subjects?.length&&<div className="empty">No assessment results are available for this term.</div>}</div>
      </div>}
    </section>

    <section className="panel"><div className="panel-head"><div><h2>Result ledger</h2><p>{results.length} recorded result{results.length===1?"":"s"}</p></div></div><div className="table-wrap"><table><thead><tr><th>Score</th><th>Assessment</th><th>Enrollment</th></tr></thead><tbody>{results.map(x=><tr key={x.id}><td><strong>{x.score}</strong></td><td>{x.assessment?.title||x.assessment_id}</td><td>{x.student_enrollment?.student?.user?.name||x.student_enrollment_id}</td></tr>)}</tbody></table>{!results.length&&<div className="empty">No results recorded.</div>}</div></section>
  </div>;
}
