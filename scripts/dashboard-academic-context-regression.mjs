import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";

const root = path.resolve(process.cwd(), "..");
const read = (file) => readFile(path.join(root, "frontend", file), "utf8");

const dashboardLayout = await read("app/dashboard/layout.tsx");
assert.doesNotMatch(dashboardLayout, /AcademicContextProvider|AcademicSelector/);

for (const file of [
  "app/dashboard/teacher/layout.tsx",
  "app/dashboard/student/layout.tsx",
  "app/dashboard/parent/layout.tsx",
]) {
  const content = await read(file);
  assert.doesNotMatch(content, /AcademicContextProvider|AcademicSelector/);
}

for (const file of [
  "app/dashboard/attendance/page.tsx",
  "app/dashboard/assessments/page.tsx",
  "app/dashboard/results/page.tsx",
  "app/dashboard/finance/page.tsx",
  "app/dashboard/timetable/page.tsx",
]) {
  const content = await read(file);
  assert.match(content, /AcademicContextProvider/);
  assert.match(content, /AcademicSelector/);
}

for (const file of [
  "app/dashboard/teacher/page.tsx",
  "app/dashboard/student/page.tsx",
  "app/dashboard/parent/page.tsx",
]) {
  const content = await read(file);
  assert.doesNotMatch(content, /useAcademicContext|AcademicContextProvider|AcademicSelector/);
}
const platformDashboard = await read("app/dashboard/page.tsx");
assert.ok(platformDashboard.includes('role === "super_admin"'));
assert.ok(platformDashboard.includes("AcademicContextProvider"));
assert.ok(platformDashboard.includes("AcademicSelector"));
assert.ok(platformDashboard.includes("/admin/dashboard"));
assert.match(platformDashboard, /loading: contextLoading/);
assert.match(platformDashboard, /if \(!contextLoading\) load\(\)/);
assert.match(platformDashboard, /first-run-setup/);
assert.match(platformDashboard, /Create academic session/);
assert.match(platformDashboard, /Add academic term/);

const academicSelector = await read("components/academic-selector.tsx");
assert.match(academicSelector, /sessions\.length === 0/);
assert.match(academicSelector, /terms\.length/);
assert.match(academicSelector, /href="\/dashboard\/academic"/);
assert.match(academicSelector, /Create academic session/);

console.log("Dashboard academic-context and first-run school setup checks passed.");
