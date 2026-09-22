#!/usr/bin/env node
const BASE_URL = (process.env.PRODUCTION_API_URL || "https://stonez-digital-school-api.onrender.com").replace(/\/$/,"");
const runId = process.env.QA_RUN_ID || new Date().toISOString().replace(/[^0-9]/g,"").slice(0,14);
const schools = JSON.parse(process.env.PRODUCTION_QA_SCHOOLS_JSON || "[]");
const superAdmin = {email:process.env.PRODUCTION_QA_SUPER_ADMIN_EMAIL,password:process.env.PRODUCTION_QA_SUPER_ADMIN_PASSWORD,school_code:""};
const results = [];
function fail(message){throw new Error(message)}
async function request(path,{token,method="GET",body,expected=[200]}={}) {
  const r=await fetch(BASE_URL+path,{method,headers:{"Content-Type":"application/json",...(token?{Authorization:"Bearer "+token}:{})},body:body===undefined?undefined:JSON.stringify(body)});
  const text=await r.text(); let data={}; try{data=text?JSON.parse(text):{}}catch{data={raw:text}};
  if(!expected.includes(r.status)) fail(`${method} ${path} expected ${expected.join("/")} got ${r.status}: ${JSON.stringify(data)}`);
  return {status:r.status,data};
}
async function login(creds){return (await request("/auth/login",{method:"POST",body:creds,expected:[200]})).data}
async function me(token){return (await request("/me",{token})).data}
function list(d,k){return Array.isArray(d)?d:(d?.[k]||d?.data||[])}
function record(area,school,role,status,notes=""){results.push({area,school,role,status,notes})}
async function ensureAcademic(token,schoolCode){
  let sessions=list((await request("/admin/academic-sessions",{token})).data,"sessions");
  let active=sessions.find(x=>x.status==="active");
  if(!active){
    const body={name:`QA ${runId} Active Session`,start_date:"2026-09-01",end_date:"2027-07-31",status:"active"};
    active=(await request("/admin/academic-sessions",{token,method:"POST",body,expected:[201]})).data;
  }
  let target=sessions.find(x=>x.id!==active.id);
  if(!target){
    target=(await request("/admin/academic-sessions",{token,method:"POST",body:{name:`QA ${runId} Target Session`,start_date:"2027-09-01",end_date:"2028-07-31",status:"planned"},expected:[201]})).data;
  }
  let classes=list((await request("/admin/classes",{token})).data,"classes");
  let cls=classes[0];
  if(!cls) cls=(await request("/admin/classes",{token,method:"POST",body:{name:`QA JSS1 ${runId}`,level:1},expected:[201]})).data;
  let sections=list((await request(`/admin/classes/${cls.id}/sections`,{token})).data,"sections");
  let sec=sections[0];
  if(!sec) sec=(await request(`/admin/classes/${cls.id}/sections`,{token,method:"POST",body:{name:`QA-A`},expected:[201]})).data;
  return {active,target,cls,sec};
}
async function bootstrapSchool(s){
  const school=s.school_code;
  const admin=await login({email:s.admin_email,password:s.admin_password,school_code:school});
  const adminMe=await me(admin.access_token);
  if(adminMe.role!=="school_admin") fail(`${school}: school admin role mismatch`);
  if(!adminMe.school_name) fail(`${school}: /me missing school_name`);
  record("authentication",school,"school_admin","PASS",adminMe.school_name);
  const academic=await ensureAcademic(admin.access_token,school);
  const stamp=`${runId}.${school.toLowerCase()}`;
  const password="QA-"+runId+"-Pass9";
  const studentEmail=`student.${stamp}@example.com`;
  const teacherEmail=`teacher.${stamp}@example.com`;
  const parentEmail=`parent.${stamp}@example.com`;
  const student=(await request("/admin/onboarding/people",{token:admin.access_token,method:"POST",body:{name:`QA Student ${runId}`,email:studentEmail,password,role:"student",admission_number:`QA-${runId}-${school}`,enrollment_status:"active"},expected:[201]})).data;
  const studentId=student.student?.id||student.id;
  const teacher=(await request("/admin/onboarding/people",{token:admin.access_token,method:"POST",body:{name:`QA Teacher ${runId}`,email:teacherEmail,password,role:"teacher"},expected:[201]})).data;
  const parent=(await request("/admin/onboarding/people",{token:admin.access_token,method:"POST",body:{name:`QA Parent ${runId}`,email:parentEmail,password,role:"parent",student_id:studentId,relationship:"parent",primary:true},expected:[201]})).data;
  record("onboarding",school,"school_admin","PASS","Created temporary teacher/student/parent");
  const enrollment=(await request("/admin/enrollments",{token:admin.access_token,method:"POST",body:{student_id:studentId,academic_session_id:academic.active.id,class_id:academic.cls.id,section_id:academic.sec.id,status:"active"},expected:[201]})).data;
  const promoted=(await request(`/admin/enrollments/${enrollment.id}/place`,{token:admin.access_token,method:"POST",body:{target_session_id:academic.target.id,target_class_id:academic.cls.id,target_section_id:academic.sec.id,operation:"promote"},expected:[201]})).data;
  const history=list((await request(`/admin/enrollments/student/${studentId}`,{token:admin.access_token})).data,"enrollments");
  if(promoted.status!=="active") fail(`${school}: promoted enrollment is not active`);
  if(history.length<2) fail(`${school}: promotion history missing source/target`);
  record("student lifecycle",school,"school_admin","PASS","Promotion created active target and preserved history");
  const studentAuth=await login({email:studentEmail,password,school_code:school});
  const teacherAuth=await login({email:teacherEmail,password,school_code:school});
  const parentAuth=await login({email:parentEmail,password,school_code:school});
  const studentMe=await me(studentAuth.access_token), teacherMe=await me(teacherAuth.access_token), parentMe=await me(parentAuth.access_token); 
  await request("/auth/refresh",{method:"POST",body:{refresh_token:studentAuth.refresh_token},expected:[200]});
  await request("/auth/logout",{method:"POST",body:{refresh_token:studentAuth.refresh_token},expected:[200]});
  await login({email:studentEmail,password,school_code:school});\n  record("logout/session",school,"student","PASS","Refresh and logout cycle completed");
  for(const [role,x] of [["student",studentMe],["teacher",teacherMe],["parent",parentMe]]) {
    if(x.role!==role||!x.school_name) fail(`${school}: ${role} /me missing role or school_name`);
    record("authentication",school,role,"PASS",x.school_name);
  }
  await request("/student/profile",{token:studentAuth.access_token});
  await request("/student/enrollment",{token:studentAuth.access_token});
  await request("/student/terms",{token:studentAuth.access_token});
  record("academic access",school,"student","PASS","Student portal profile/enrollment/terms");
  const children=list((await request("/parent/children",{token:parentAuth.access_token})).data,"children");
  if(!children.some(x=>x.student_id===studentId)) fail(`${school}: parent child link missing`);
  record("academic access",school,"parent","PASS","Linked child visible");
  await request("/teacher/assignments",{token:teacherAuth.access_token});
  record("academic access",school,"teacher","PASS","Teacher assignments endpoint accessible");
  await request("/admin/users",{token:studentAuth.access_token,expected:[403]});
  record("role isolation",school,"student","PASS","Student denied school-admin users endpoint");
  await request("/me",{expected:[401]});
  record("unauthorized API",school,"anonymous","PASS","Protected /me returned 401");
  return {school,admin,studentAuth,teacherAuth,parentAuth,studentId,adminMe};
}
if(!superAdmin.email||!superAdmin.password) fail("Missing super-admin QA credentials");
if(schools.length!==2) fail("PRODUCTION_QA_SCHOOLS_JSON must contain exactly two active school-admin credential objects");
const superAuth=await login(superAdmin);
const superMe=await me(superAuth.access_token);
if(superMe.role!=="super_admin") fail("Super Admin QA account is not super_admin");
record("authentication","platform","super_admin","PASS","Platform profile loaded");
const contexts=[];
for(const s of schools) contexts.push(await bootstrapSchool(s));
const a=contexts[0], b=contexts[1];
const bUsers=list((await request("/admin/users",{token:b.admin.access_token})).data,"users");
const bStudents=list((await request("/admin/students",{token:b.admin.access_token})).data,"students");
if(bUsers[0]) await request(`/admin/users/${bUsers[0].id}`,{token:a.admin.access_token,expected:[404]});
if(bStudents[0]) await request(`/admin/students/${bStudents[0].id}`,{token:a.admin.access_token,expected:[404]});
record("tenant isolation",a.school,"school_admin","PASS","Cross-school user/student IDs rejected");
const aSessions=list((await request("/admin/academic-sessions",{token:a.admin.access_token})).data,"sessions");
const bSessions=list((await request("/admin/academic-sessions",{token:b.admin.access_token})).data,"sessions");
if(bSessions[0]) await request(`/admin/academic-sessions/${bSessions[0].id}`,{token:a.admin.access_token,expected:[404]});
record("tenant isolation",a.school,"school_admin","PASS","Cross-school academic session rejected");
console.log(JSON.stringify({run_id:runId,base_url:BASE_URL,results,temporary_accounts:{created_by_school:contexts.map(x=>x.school)},note:"Credentials are generated at runtime and are never printed or persisted by this script."},null,2));
