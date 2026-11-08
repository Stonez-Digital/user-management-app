"use client";

import {useEffect,useMemo,useState} from "react";
import Link from "next/link";
import { AcademicContextProvider, useAcademicContext } from "../../../lib/academic-context";
import AcademicSelector from "../../../components/academic-selector";

type RecordItem=Record<string,any>;
async function api(path:string,options:RequestInit={}){const token=localStorage.getItem("access_token");const r=await fetch("/backend"+path,{...options,headers:{...options.headers,Authorization:"Bearer "+token,"Content-Type":"application/json"}});const d=await r.json().catch(()=>({}));if(r.status===401)throw Error("Session expired");if(!r.ok)throw Error(d?.error?.message||"Request failed");return d}
const arr=(d:any,key:string)=>Array.isArray(d)?d:d?.[key]||d?.data||[];
const money=(value:any)=>new Intl.NumberFormat("en-NG",{style:"currency",currency:"NGN",maximumFractionDigits:2}).format(Number(value||0));

function FinanceContent(){
 const { sessionId: selectedSessionId, termId: selectedTermId, terms: selectedTerms } = useAcademicContext();
 const[fees,setFees]=useState<RecordItem[]>([]),[invoices,setInvoices]=useState<RecordItem[]>([]),[terms,setTerms]=useState<RecordItem[]>([]),[enrollments,setEnrollments]=useState<RecordItem[]>([]),[error,setError]=useState(""),[busy,setBusy]=useState(false);
 const[receipt,setReceipt]=useState<RecordItem|null>(null),[ledgerSearch,setLedgerSearch]=useState(""),[ledgerStatus,setLedgerStatus]=useState("all"),[ledgerFrom,setLedgerFrom]=useState(""),[ledgerTo,setLedgerTo]=useState("");
 const[fee,setFee]=useState({term_id:"",name:"",description:"",amount:""});
 const[invoice,setInvoice]=useState({student_enrollment_id:"",term_id:"",due_date:"",fee_item_id:"",description:"",quantity:"1",unit_amount:""});
 const[payment,setPayment]=useState({invoice_id:"",amount:"",provider:"manual",reference:""});
 const selectedInvoice=useMemo(()=>invoices.find(x=>x.id===payment.invoice_id),[invoices,payment.invoice_id]);
 const ledger=useMemo(()=>invoices.flatMap((inv:RecordItem)=>(Array.isArray(inv.payments)?inv.payments:[]).map((p:RecordItem)=>{const enrollment=inv.student_enrollment||{},student=enrollment.student||{};return {...p,invoice:inv,invoice_number:inv.invoice_number||inv.id,student_admission:student.admission_number||"—",student_name:inv.student_name||student.name||"Student name unavailable",student_email:student.user?.email||"",class_name:inv.student_class_name||enrollment.class?.name||enrollment.school_class?.name||"—",term_name:inv.term?.name||"—"} as RecordItem})).sort((a:RecordItem,b:RecordItem)=>new Date(b.paid_at||b.created_at||0).getTime()-new Date(a.paid_at||a.created_at||0).getTime()),[invoices]);
 const ledgerTotals=useMemo(()=>ledger.reduce((totals,p)=>{const amount=Number(p.amount||0);if(p.status==="succeeded")totals.collected+=amount;else if(p.status==="pending")totals.pending+=amount;else if(p.status==="refunded")totals.refunded+=amount;return totals},{collected:0,pending:0,refunded:0}),[ledger]);
 const visibleLedger=useMemo(()=>ledger.filter((p:RecordItem)=>{const q=ledgerSearch.trim().toLowerCase(),date=new Date(p.paid_at||p.created_at||0).toISOString().slice(0,10);const matches=!q||[p.receipt_number,p.reference,p.invoice_number,p.student_admission,p.student_name,p.student_email,p.class_name,p.provider].some(v=>String(v||"").toLowerCase().includes(q));return matches&&(ledgerStatus==="all"||p.status===ledgerStatus)&&(!ledgerFrom||date>=ledgerFrom)&&(!ledgerTo||date<=ledgerTo)}),[ledger,ledgerSearch,ledgerStatus,ledgerFrom,ledgerTo]);
 function exportLedger(){const columns=["receipt_number","date","student_name","student_admission","class_name","invoice_number","term_name","provider","reference","status","amount"];const rows=visibleLedger.map(p=>[p.receipt_number||"",p.paid_at||p.created_at||"",p.student_name,p.student_admission,p.class_name,p.invoice_number,p.term_name,p.provider,p.reference,p.status,p.amount]);const csv=[columns,...rows].map(row=>row.map(value=>`"${String(value??"").replace(/"/g,'""')}"`).join(",")).join("\r\n");const blob=new Blob(["\uFEFF",csv],{type:"text/csv;charset=utf-8;"});const url=URL.createObjectURL(blob);const anchor=document.createElement("a");anchor.href=url;anchor.download="school-payment-ledger.csv";anchor.click();URL.revokeObjectURL(url)}

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
 <section className="panel" id="record-payment"><div className="panel-head"><div><h2>Record payment</h2><p>Payments are validated against the invoice balance and remain school-scoped.</p></div></div><div className="form-grid">
  <label>Invoice<select value={payment.invoice_id}onChange={e=>{const id=e.target.value;const inv=invoices.find(x=>x.id===id);setPayment({...payment,invoice_id:id,amount:inv?.balance?String(inv.balance):""})}}><option value="">Select invoice</option>{invoices.filter(x=>Number(x.balance||0)>0).map(x=><option key={x.id}value={x.id}>{x.invoice_number||x.id} — balance {x.balance}</option>)}</select></label>
  <label>Amount<input type="number"min="0.01"step="0.01"value={payment.amount}onChange={e=>setPayment({...payment,amount:e.target.value})}/></label><label>Provider<input value={payment.provider}onChange={e=>setPayment({...payment,provider:e.target.value})}/></label><label>Reference<input value={payment.reference}onChange={e=>setPayment({...payment,reference:e.target.value})}placeholder="Receipt/reference"/></label>
  <div className="form-action"><button disabled={busy||!payment.invoice_id||!payment.amount||!payment.reference}onClick={()=>submit("/admin/invoices/"+payment.invoice_id+"/payments",{amount:Number(payment.amount),provider:payment.provider,reference:payment.reference,status:"succeeded"},()=>setPayment({invoice_id:"",amount:"",provider:"manual",reference:""}))}>{busy?"Saving...":"Record payment"}</button></div>
 </div>{selectedInvoice&&<div className="empty">Selected invoice balance: {selectedInvoice.balance}</div>}</section>
 <section className="panel"><div className="panel-head"><div><h2>Fee items</h2><p>Term-scoped school charges.</p></div></div><Table rows={fees}cols={["name","amount","term_id","active"]}/></section>
 <section className="panel"><div className="panel-head"><div><h2>Invoices</h2><p>Student billing and outstanding balances.</p></div></div><Table rows={invoices}cols={["invoice_number","total_amount","paid_amount","balance","status"]}/></section>
 <section className="panel">
  <div className="panel-head"><div><h2>Payment ledger</h2><p>School-scoped payment records from the selected academic term. Only persisted payments are shown.</p></div><div className="ledger-tools"><strong>{visibleLedger.length} of {ledger.length} records</strong><button type="button" className="secondary" onClick={exportLedger} disabled={!visibleLedger.length}>Export CSV</button></div></div>
  <div className="ledger-summary"><Stat label="Collected" value={money(ledgerTotals.collected)} detail="Succeeded payments"/><Stat label="Pending" value={money(ledgerTotals.pending)} detail="Awaiting confirmation"/><Stat label="Refunded" value={money(ledgerTotals.refunded)} detail="Recorded refunds"/></div>
  <div className="form-grid ledger-filters">
   <label>Search payments<input value={ledgerSearch} onChange={e=>setLedgerSearch(e.target.value)} placeholder="Student, class, receipt, reference or invoice"/></label>
   <label>Payment status<select value={ledgerStatus} onChange={e=>setLedgerStatus(e.target.value)}><option value="all">All statuses</option><option value="succeeded">Succeeded</option><option value="pending">Pending</option><option value="failed">Failed</option><option value="refunded">Refunded</option></select></label>
   <label>From date<input type="date" value={ledgerFrom} onChange={e=>setLedgerFrom(e.target.value)}/></label><label>To date<input type="date" min={ledgerFrom||undefined} value={ledgerTo} onChange={e=>setLedgerTo(e.target.value)}/></label>
  </div>
  <div className="table-wrap"><table><thead><tr><th>Receipt</th><th>Date</th><th>Student</th><th>Admission no.</th><th>Class</th><th>Invoice</th><th>Reference</th><th>Amount</th><th>Status</th><th>Action</th></tr></thead><tbody>
   {visibleLedger.map((p:RecordItem)=><tr key={p.id}><td>{p.receipt_number||"—"}</td><td>{p.paid_at?new Date(p.paid_at).toLocaleDateString("en-NG"):new Date(p.created_at).toLocaleDateString("en-NG")}</td><td>{p.student_name}</td><td>{p.student_admission}</td><td>{p.class_name}</td><td>{p.invoice_number}</td><td>{p.reference}</td><td>{money(p.amount)}</td><td><span className={"payment-status status-"+p.status}>{p.status}</span></td><td><button type="button" className="secondary" onClick={()=>setReceipt(p)}>View receipt</button></td></tr>)}
  </tbody></table>{visibleLedger.length===0&&<div className="empty">{ledger.length===0?<>No payments have been recorded for this academic term yet. When the school records a real payment, it will appear here. <a href="#record-payment">Go to Record payment →</a></>:<>No payments match these filters. Try clearing the search, status or date range.</>}</div>}</div>
 </section>
 {receipt&&<section className="panel receipt-print">
  <div className="receipt-head"><div><p className="eyebrow">OFFICIAL PAYMENT RECEIPT</p><h2>Payment receipt</h2><p className="muted">Keep this receipt for your records.</p></div><strong>{receipt.receipt_number||"Receipt pending"}</strong></div>
  <div className="receipt-grid">
   <div><span>Receipt number</span><strong>{receipt.receipt_number||"—"}</strong></div>
   <div><span>Payment date</span><strong>{receipt.paid_at?new Date(receipt.paid_at).toLocaleString("en-NG"):new Date(receipt.created_at).toLocaleString("en-NG")}</strong></div>
   <div><span>Invoice number</span><strong>{receipt.invoice_number}</strong></div>
   <div><span>Student name</span><strong>{receipt.student_name}</strong></div>
   <div><span>Student admission number</span><strong>{receipt.student_admission}</strong></div>
   <div><span>Class</span><strong>{receipt.class_name}</strong></div>
   <div><span>Academic term</span><strong>{receipt.term_name}</strong></div>
   <div><span>Payment method</span><strong>{receipt.provider||"—"}</strong></div>
   <div><span>Payment reference</span><strong>{receipt.reference||"—"}</strong></div>
   <div><span>Payment status</span><strong>{receipt.status}</strong></div>
  </div>
  <div className="receipt-total"><span>Amount received</span><strong>{money(receipt.amount)}</strong></div>
  <p className="receipt-note">This receipt reflects a payment record stored by the school management system. It is not a bank or payment-provider settlement confirmation.</p>
  <div className="receipt-actions"><button type="button" onClick={()=>window.print()}>Print receipt</button><button type="button" className="secondary" onClick={()=>setReceipt(null)}>Close receipt</button></div>
 </section>}
 <style jsx global>{`
  @media print {
   @page { size: A4; margin: 18mm; }
   body * { visibility: hidden !important; }
   .receipt-print, .receipt-print * { visibility: visible !important; }
   .receipt-print { position: absolute !important; inset: 0 auto auto 0 !important; width: 100% !important; margin: 0 !important; border: 0 !important; box-shadow: none !important; background: #fff !important; color: #111 !important; }
   .receipt-actions { display: none !important; }
   .receipt-print .receipt-grid { display: grid !important; grid-template-columns: 1fr 1fr !important; gap: 18px !important; }
   .receipt-print .receipt-grid span, .receipt-print .receipt-note, .receipt-print .receipt-head .muted { color: #444 !important; }
  }
  .ledger-filters { margin-bottom: 18px; }
  .ledger-tools { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
  .ledger-summary { display: grid; grid-template-columns: repeat(3,minmax(0,1fr)); gap: 14px; margin: 16px 0 22px; }
  @media (max-width: 640px) { .ledger-summary { grid-template-columns: 1fr; } }
  .payment-status { display: inline-block; text-transform: capitalize; border-radius: 999px; padding: 4px 9px; font-size: 12px; background: #eee; }
  .status-succeeded { background: #e6f5ec; color: #17663a; }
  .status-pending { background: #fff3d6; color: #805500; }
  .status-failed, .status-refunded { background: #fde8e8; color: #8c2525; }
  .receipt-print { max-width: 820px; margin: 24px auto; padding: 28px; }
  .receipt-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 20px; border-bottom: 1px solid #ddd; padding-bottom: 20px; }
  .receipt-head h2 { margin: 4px 0; }
  .receipt-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 20px; padding: 24px 0; }
  .receipt-grid div { display: flex; flex-direction: column; gap: 5px; }
  .receipt-grid span, .receipt-total span { color: #666; font-size: 13px; }
  .receipt-total { display: flex; justify-content: space-between; align-items: center; border-top: 1px solid #ddd; border-bottom: 1px solid #ddd; padding: 18px 0; }
  .receipt-total strong { font-size: 24px; }
  .receipt-note { color: #666; font-size: 12px; margin-top: 20px; }
  .receipt-actions { display: flex; gap: 10px; margin-top: 22px; }
  @media (max-width: 640px) { .receipt-print { padding: 18px; } .receipt-head { flex-direction: column; } .receipt-grid { grid-template-columns: 1fr; } }
 `}</style>
 </div>
}
function Stat({label,value,detail}:{label:string;value:string|number;detail:string}){return <div className="stat"><span>{label}</span><strong>{value}</strong><small>{detail}</small></div>}
function Panel(p:{title:string;text:string;children:React.ReactNode}){return <section className="panel"><div className="panel-head"><div><h2>{p.title}</h2><p>{p.text}</p></div></div>{p.children}</section>}
function Table({rows,cols}:{rows:RecordItem[];cols:string[]}){return <div className="table-wrap"><table><thead><tr>{cols.map(c=><th key={c}>{c}</th>)}</tr></thead><tbody>{rows.map(x=><tr key={x.id}>{cols.map(c=><td key={c}>{String(x[c]??"—")}</td>)}</tr>)}</tbody></table>{!rows.length&&<div className="empty">No records.</div>}</div>}


export default function Finance(){return <AcademicContextProvider><AcademicSelector/><FinanceContent/></AcademicContextProvider>;}
