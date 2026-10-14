import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { canRoleAccessPath } from "../frontend/lib/dashboard-access.mjs";

const studentAllowed = [
  "/dashboard",
  "/dashboard/student",
  "/dashboard/student/careerpath",
  "/dashboard/student/report-card",
  "/dashboard/notifications",
];
const studentDenied = [
  "/dashboard/students",
  "/dashboard/enrollments",
  "/dashboard/teacher-assignments",
  "/dashboard/users",
  "/dashboard/onboarding",
  "/dashboard/operations",
  "/dashboard/audit",
  "/dashboard/finance",
  "/dashboard/attendance",
  "/dashboard/assessments",
  "/dashboard/results",
  "/dashboard/timetable",
  "/dashboard/curriculum",
];

for (const path of studentAllowed) assert.equal(canRoleAccessPath("student", path), true, `student should access ${path}`);
for (const path of studentDenied) assert.equal(canRoleAccessPath("student", path), false, `student must not access ${path}`);

assert.equal(canRoleAccessPath("teacher", "/dashboard/teacher"), true);
assert.equal(canRoleAccessPath("teacher", "/dashboard/attendance"), true);
assert.equal(canRoleAccessPath("teacher", "/dashboard/users"), false);
assert.equal(canRoleAccessPath("parent", "/dashboard/parent"), true);
assert.equal(canRoleAccessPath("parent", "/dashboard/enrollments"), false);
assert.equal(canRoleAccessPath("accountant", "/dashboard/finance"), true);
assert.equal(canRoleAccessPath("staff", "/dashboard/operations"), true);
assert.equal(canRoleAccessPath("unknown", "/dashboard"), false);
assert.equal(canRoleAccessPath("school_admin", "/dashboard/users"), true);
assert.equal(canRoleAccessPath("super_admin", "/dashboard/audit"), true);

const shell = await readFile(new URL("../frontend/components/role-access-shell.tsx", import.meta.url), "utf8");
assert.ok(shell.includes('fetch("/backend/me"'));
assert.ok(shell.includes("router.replace(landingPathForRole(role))"));
assert.ok(shell.includes("if (!ready)"));
console.log("Role-aware dashboard access regression checks passed.");
