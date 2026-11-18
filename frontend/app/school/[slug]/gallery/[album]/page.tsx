"use client";
import {useEffect,useState}from"react";
import {useParams}from"next/navigation";
import Link from"next/link";
export default function Album(){
 const{slug,album}=useParams<{slug:string;album:string}>();const[d,setD]=useState<any>();const[e,setE]=useState("");const[light,setLight]=useState<any>();
 useEffect(()=>{fetch("/backend/public/schools/"+encodeURIComponent(String(slug))+"/gallery/"+encodeURIComponent(String(album))).then(async r=>{const x=await r.json();if(!r.ok)throw Error(x?.error||"Album not found");setD(x)}).catch(x=>setE(x.message))},[slug,album]);
 if(e)return <main className="content"><div className="error banner">{e}</div></main>;
 if(!d)return <main className="content"><div className="panel">Loading album…</div></main>;
 return <main className="content" style={{maxWidth:1180,margin:"0 auto"}}><Link href={"/school/"+slug+"/gallery"}>← Gallery</Link><header className="topbar"><div><p className="eyebrow">PHOTO ALBUM</p><h1>{d.album.title}</h1><p className="muted">{d.album.description}</p></div></header><div style={{display:"grid",gridTemplateColumns:"repeat(auto-fill,minmax(220px,1fr))",gap:14}}>{(d.images||[]).map((x:any)=><button key={x.id} onClick={()=>setLight(x)} style={{border:0,padding:0,background:"none",cursor:"pointer"}}><img src={x.image_url} alt={x.alt_text||x.caption||d.album.title} style={{width:"100%",height:220,objectFit:"cover",borderRadius:12}}/></button>)}</div>{light&&<div onClick={()=>setLight(null)} style={{position:"fixed",inset:0,background:"rgba(0,0,0,.85)",display:"grid",placeItems:"center",zIndex:100,padding:20}}><img src={light.image_url} alt={light.alt_text||light.caption||""} style={{maxWidth:"95vw",maxHeight:"90vh",objectFit:"contain"}}/></div>}</main>;
}