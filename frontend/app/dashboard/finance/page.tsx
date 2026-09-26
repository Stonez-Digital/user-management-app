"use client";

import {useEffect,useMemo,useState} from "react";
import Link from "next/link";
import { AcademicContextProvider, useAcademicContext } from "../../../lib/academic-context";
import AcademicSelector from "../../../components/academic-selector";

type RecordItem=Record<string,any>;
async function api(path:string,options:RequestInit={}){const token=localStorage.getItem("access_token");const r=await fetch("/backend"+path,{...options,headers:{...options.headers,Authorization:"Bearer "+token,"Content-Type":"application/json"}});const d=await r.json().catch(()=>({}));if(r.status===401)throw Error("Session expired");if(!r.ok)throw Error(d?.error?.message||"Request failed");return d}
const arr=(d:any,key:string)=>Array.isArray(d)?d:d?.[key]||d?.data||[];

function FinanceContent(){
 const { sessionId: selectedSessionId, termId: selectedTermId, terms: selectedTerms } = useAcademicContext();
 const[fees,setFees]=useState<RecordItem[]>([]),[invoices,setInvoices]=useState<RecordItem[]>([]),[terms,setTerms]=useState<RecordItem[]>([]),[enrollments,setEnrollments]=useState<RecordItem[]>([]),[error,setError]=useState(""),[busy,setBusy]=useState(false);
 const[fee,setFee]=useState({term_id:"",name:"",description:"",amount:""});
 const[invoice,setInvoice]=useState({student_enrollment_id:"",term_id:"",due_date:"",fee_item_id:"",description:"",quantity:"1",unit_amount:""});
 const[payment,setPayment]=useState({invoice_id:"",amount:"",provider:"manual",reference:""});
 const selectedInvoice=useMemo(()=>invoices.find(x=>x.id===payment.invoice_id),[invoices,payment.invoice_id]);

 async function load(){
  if(!selectedSessionId||!selectedTermId)return;
  try{
   setError("");
   const[f,i,e]=await Promise.all([api("/admin/fees?academic_session_id="+selectedSessionId+"&term_id="+selectedTermId),api("/admin/invoices?academic_session_id="+selectedSessionId+"&term_id="+selectedTermId),api("/admin/enrollments?academic_session_id="+selectedSessionId)]);
   const scopedFees=arr(f,"fees").filter((x:RecordItem)=>!selectedTermId||x.term_id===selectedTermId);
   const scopedInvoices=arr(i,"invoices").filter((x:RecordItem)=>!selectedTermId||x.term_id===selectedTermId);
   const scopedEnrollments=arr(e,"enrollments").filter((x:RecordItem)=>(!selectedSessionId||x.academic_session_id===selectedSessionId||x.academic_session?.id===selectedSessionId)&&String(x.status||"").toLowerCase()==="active");
   setFees(scopedFees);setInvoices(scopedInvoices);setEnrollments(scopedEnrollments);setTerms(selectedTerms);
   if(selectedTermId){setFee(x=>({...x,term_id:selectedTermId}));setInvoice(x=>({...x,term_id:selectedTermId}))}
  }catch(x:any){setError(x.message||"Unable to load finance data")}
 }
 useEffect(()=>{load()},[selectedSessionId,selectedTermId,selectedTerms]);
 async function submit(path:string,body:any,reset:()=>void){setBusy(true);setError("");try{await api(path,{method:"POST",body:JSON.stringify(body)});reset();await load()}catch(x:any){setError(x.message||"Finance operation failed")}finally{setBusy(false)}}

 return <div className="content standalone"><Link className="back"href="/dashboard/operations">← Operations</Link>
 <header className="topbar"><div><p className="eyebrow">FINANCE ADMINISTRATION</p><h1>Fees, invoices & payments</h1><p className="muted">Manage school-scoped charges, student billing and payment records.</p></div></header>
 {error&&<div className="error banner">{error}</div>}
 <section className="stats"><Stat label="Fee items" value={fees.length} detail="Configured charges"/><Stat label="Invoices" value={invoices.length} detail="Student billing"/><Stat label="Outstanding" value={invoices.reduce((n,x)=>n+Number(x.balance||0),0).toFixed(2)} detail="Current balance"/><Stat label="Paid invoices" value={invoices.filter(x=>x.status==="paid").length} detail="Fully settled"/></section>
 <section className="grid-2">
  <Panel title="Create fee item" text="Charges are isolated to the selected academic term and school."><div className="form-grid">
   <label>Term<select value={fee.term_id}onChange={e=>setFee({...fee,term_id:e.target.value})}><option value="">Select term</option>{terms.map(t=><option key={t.id}value={t.id}>{t.name}</option>)}</select></label>
   <label>Name<input value={fee.name}onChange={e=>setFee({...fee,name:e.target.value})}placeholder="Tuition"/></label>
   <label>Amount<input type="number"min="0"step="0.01"value={fee.amount}onChange={e=>setFee({...fee,amount:e.target.value})}placeholder="50000"/></label>
   <label>Description<input value={fee.description}onChange={e=>setFee({...fee,description:e.target.value})}placeholder="Term tuition"/></label>
   <div className="form-action"><button disabled={busy||!fee.term_id||!fee.name||!fee.amount}onClick={()=>submit("/admin/terms/"+fee.term_id+"/fees",{name:fee.name,description:fee.description,amount:Number(fee.amount),active:true},()=>setFee({...fee,name:"",description:"",amount:""}))}>{busy?"Saving...":"Create fee"}</button></div>
  </div></Panel>
  <Panel title="Create invoice" text="Invoice an active student enrollment using a selected term.">{enrollments.length===0&&<div className="empty">No active student enrollments are available for the selected academic session. A student must first be placed in a class and section for this session. <Link href="/dashboard/enrollments">Go to Student Enrollment →</Link></div>}<form className="form-grid" onSubmit={e=>{e.preventDefault();if(!invoice.student_enrollment_id||!invoice.term_id||!invoice.due_date||!invoice.description||Number(invoice.quantity)<=0||Number(invoice.unit_amount)<=0){setError("Complete all invoice fields before creating the invoice.");return}setError("");submit("/admin/invoices",{student_enrollment_id:invoice.student_enrollment_id,term_id:invoice.term_id,due_date:invoice.due_date,lines:[{fee_item_id:invoice.fee_item_id||undefined,description:invoice.description.trim(),quantity:Number(invoice.quantity),unit_amount:Number(invoice.unit_amount)}]},()=>setInvoice({...invoice,student_enrollment_id:"",due_date:"",fee_item_id:"",description:"",quantity:"1",unit_amount:""}))}}>
   <label>Student enrollment<select required value={invoice.student_enrollment_id}onChange={e=>setInvoice({...invoice,student_enrollment_id:e.target.value})}><option value="">Select active enrollment</option>{enrollments.map(x=><option key={x.id}value={x.id}>{x.student?.first_name||x.student?.user?.first_name||x.student?.last_name||x.student_id||x.id} — {x.class?.name||x.class_name||"enrollment"}</option>)}</select></label>
   <label>Term<select required value={invoice.term_id}onChange={e=>{const id=e.target.value;const t=terms.find(x=>x.id===id);setInvoice({...invoice,term_id:id,due_date:t?.end_date?t.end_date.slice(0,10):invoice.due_date})}}><option value="">Select term</option>{terms.map(t=><option key={t.id}value={t.id}>{t.name}</option>)}</select></label>
   <label>Fee item<select value={invoice.fee_item_id} onChange={e=>{const id=e.target.value;const item=fees.find(x=>x.id===id);setInvoice({...invoice,fee_item_id:id,description:item?.name||invoice.description,unit_amount:item?.amount!=null?String(item.amount):invoice.unit_amount})}}><option value="">Select fee item or enter manually</option>{fees.filter(x=>(!invoice.term_id||x.term_id===invoice.term_id)&&x.active!==false).map(x=><option key={x.id} value={x.id}>{x.name} — {x.amount}</option>)}</select></label>
   <label>Due date<input required type="date"value={invoice.due_date}onChange={e=>setInvoice({...invoice,due_date:e.target.value})}/></label>
   <label>Description<input required value={invoice.description}onChange={e=>setInvoice({...invoice,description:e.target.value})}placeholder="Tuition / school fees"/></label>
   <label>Quantity<input required type="number"min="0.01"step="0.01"value={invoice.quantity}onChange={e=>setInvoice({...invoice,quantity:e.target.value})}/></label>
   <label>Unit amount<input required type="number"min="0.01"step="0.01"value={invoice.unit_amount}onChange={e=>setInvoice({...invoice,unit_amount:e.target.value})}placeholder="0.00"/></label>
   <div className="form-action"><button type="submit" disabled={busy||enrollments.length===0}>{busy?"Creating invoice…":"Create invoice"}</button></div>
  </form></Panel>
 </section>
 <section className="panel"><div className="panel-head"><div><h2>Record payment</h2><p>Payments are validated against the invoice balance and remain school-scoped.</p></div></div><div className="form-grid">
  <label>Invoice<select value={payment.invoice_id}onChange={e=>{const id=e.target.value;const inv=invoices.find(x=>x.id===id);setPayment({...payment,invoice_id:id,amount:inv?.balance?String(inv.balance):""})}}><option value="">Select invoice</option>{invoices.filter(x=>Number(x.balance||0)>0).map(x=><option key={x.id}value={x.id}>{x.invoice_number||x.id} — balance {x.balance}</option>)}</select></label>
  <label>Amount<input type="number"min="0.01"step="0.01"value={payment.amount}onChange={e=>setPayment({...payment,amount:e.target.value})}/></label><label>Provider<input value={payment.provider}onChange={e=>setPayment({...payment,provider:e.target.value})}/></label><label>Reference<input value={payment.reference}onChange={e=>setPayment({...payment,reference:e.target.value})}placeholder="Receipt/reference"/></label>
  <div className="form-action"><button disabled={busy||!payment.invoice_id||!payment.amount||!payment.reference}onClick={()=>submit("/admin/invoices/"+payment.invoice_id+"/payments",{amount:Number(payment.amount),provider:payment.provider,reference:payment.reference,status:"succeeded"},()=>setPayment({invoice_id:"",amount:"",provider:"manual",reference:""}))}>{busy?"Saving...":"Record payment"}</button></div>
 </div>{selectedInvoice&&<div className="empty">Selected invoice balance: {selectedInvoice.balance}</div>}</section>
 <section className="panel"><div className="panel-head"><div><h2>Fee items</h2><p>Term-scoped school charges.</p></div></div><Table rows={fees}cols={["name","amount","term_id","active"]}/></section>
 <section className="panel"><div className="panel-head"><div><h2>Invoices</h2><p>Student billing and outstanding balances.</p></div></div><Table rows={invoices}cols={["invoice_number","total_amount","paid_amount","balance","status"]}/></section>
 </div>
}
function Stat({label,value,detail}:{label:string;value:string|number;detail:string}){return <div className="stat"><span>{label}</span><strong>{value}</strong><small>{detail}</small></div>}
function Panel(p:{title:string;text:string;children:React.ReactNode}){return <section className="panel"><div className="panel-head"><div><h2>{p.title}</h2><p>{p.text}</p></div></div>{p.children}</section>}
function Table({rows,cols}:{rows:RecordItem[];cols:string[]}){return <div className="table-wrap"><table><thead><tr>{cols.map(c=><th key={c}>{c}</th>)}</tr></thead><tbody>{rows.map(x=><tr key={x.id}>{cols.map(c=><td key={c}>{String(x[c]??"—")}</td>)}</tr>)}</tbody></table>{!rows.length&&<div className="empty">No records.</div>}</div>}


export default function Finance(){return <AcademicContextProvider><AcademicSelector/><FinanceContent/></AcademicContextProvider>;}
