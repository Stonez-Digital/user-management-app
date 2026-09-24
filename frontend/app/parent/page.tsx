"use client";

import {useEffect} from "react";
import {useRouter} from "next/navigation";

export default function ParentPortalRedirect(){
  const router=useRouter();
  useEffect(()=>{router.replace("/dashboard/parent")},[router]);
  return <main className="content standalone"><div className="panel"><h2>Loading parent portal...</h2></div></main>;
}
