"use client";
import {useEffect,useState} from "react";
import Link from "next/link";
import {useSearchParams} from "next/navigation";
type Item=Record<string,any>;
async function api(path:string){const t=localStorage.getItem("access_token");const r=await fetch("/backend"+path,{headers:{Authorization:"Bearer "+t}});const d=await r.json().catch(()=>({}));if(!r.ok)throw Error(d?.error?.message||"Request failed");return d}
export default function StudentReportCard(){const q=useSearchParams(),termId=q.get("termId"),[data,setData]=useState<Item|null>(null),[error,setError]=useState("");
useEffect(()=>{if(!termId)return;(async()=>{try{setData(await api("/student/report-cards/"+termId))}catch(e:any){setError(e.message||"Unable to load report card")}})()},[termId]);
const rows=(data?.subjects||data?.results||data?.items||[]) as Item[];
return <div className="content standalone"><Link className="back" href="/dashboard/student">← Student portal</Link><header className="topbar"><div><p className="eyebrow">REPORT CARD</p><h1>Academic results</h1><p className="muted">Term performance for your current enrollment.</p></div></header>{error&&<div className="error banner">{error}</div>}{data&&<><section className="stats"><Stat label="Overall percentage" value={Number(data.overall_percentage??data.percentage??0)}/><Stat label="Assessments" value={Number(data.assessment_count??rows.reduce((n,x)=>n+Number(x.assessment_count||0),0))}/></section><section className="panel"><div className="panel-head"><div><h2>Subject results</h2></div></div><div className="table-wrap"><table><thead><tr><th>Subject</th><th>Total</th><th>Percentage</th><th>Weighted contribution</th></tr></thead><tbody>{rows.map((x,i)=><tr key={x.subject_id||x.id||i}><td>{x.subject_name||x.name||"—"}</td><td>{String(x.total??x.score??"—")}</td><td>{String(x.percentage??"—")}</td><td>{String(x.weighted_contribution??"—")}</td></tr>)}</tbody></table>{!rows.length&&<div className="empty">No results for this term.</div>}</div></section></>}</div>}
function Stat({label,value}:{label:string;value:number}){return <div className="stat"><span>{label}</span><strong>{value}</strong></div>}
