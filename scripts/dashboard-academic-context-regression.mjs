import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";

const root = path.resolve(process.cwd(), "..");
const read = (file) => readFile(path.join(root, "frontend", file), "utf8");

const teacher = await read("app/dashboard/teacher/page.tsx");
assert.match(teacher, /useEffect\(\(\)=>\{if\(!selectedSessionId\|\|!selectedTermId\)return;/);

const student = await read("app/dashboard/student/page.tsx");
assert.match(student, /useEffect\(\(\)=>\{if\(!sessionId\|\|!termId\)return;/);

const parentDashboard = await read("app/dashboard/parent/page.tsx");
assert.match(parentDashboard, /if\(!selected\|\|!sessionId\|\|!termId\)return;/);
assert.match(parentDashboard, /\/parent\/terms\?student_id=\"\+id\+\"&academic_session_id=\"\+sessionId/);

const parentLegacy = await read("app/parent/page.tsx");
assert.doesNotMatch(parentLegacy, /\/parent\/terms(?!\?)/);
assert.ok(parentLegacy.includes('router.replace("/dashboard/parent")'));

const studentLegacy = await read("app/student/page.tsx");
assert.doesNotMatch(studentLegacy, /\/student\/(?:enrollment|attendance|timetable|invoices|payments|terms)(?!\?)/);
assert.ok(studentLegacy.includes('router.replace("/dashboard/student")'));

const auth = await read("lib/auth-session.ts");
assert.match(auth, /case "parent":\s*return "\/dashboard\/parent";/);
assert.match(auth, /case "student":\s*return "\/dashboard\/student";/);

console.log("Dashboard academic-context regression checks passed.");
