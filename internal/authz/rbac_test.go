package authz

import "testing"

func TestSchoolRolesAreValid(t *testing.T) {
    roles:=[]string{RoleSuperAdmin,RoleSchoolAdmin,RoleTeacher,RoleStudent,RoleParent,RoleAccountant,RoleStaff}
    for _,role:=range roles{if !IsValidRole(role){t.Fatalf("expected role %q to be valid",role)}}
}
func TestSuperAdminHasAllDefinedPermissions(t *testing.T) {
    permissions:=[]string{PermissionUsersRead,PermissionUsersCreate,PermissionUsersUpdate,PermissionUsersDelete,PermissionUsersActivate,PermissionUsersDeactivate,PermissionRolesAssign,PermissionEnrollmentRead,PermissionEnrollmentManage,PermissionAttendanceRead,PermissionAttendanceManage}
    for _,permission:=range permissions{if !HasPermission(RoleSuperAdmin,permission){t.Fatalf("super admin should have %q",permission)}}
}
func TestSchoolAdminCannotAssignRolesOrDeleteUsers(t *testing.T) {
    if HasPermission(RoleSchoolAdmin,PermissionRolesAssign){t.Fatal("school admin must not assign roles")}
    if HasPermission(RoleSchoolAdmin,PermissionUsersDelete){t.Fatal("school admin must not delete users")}
    if !HasPermission(RoleSchoolAdmin,PermissionUsersRead){t.Fatal("school admin should read users")}
}
func TestNonAdminRolesHaveNoAdministrativePermissions(t *testing.T) {
    roles:=[]string{RoleTeacher,RoleStudent,RoleParent,RoleAccountant,RoleStaff}
    permissions:=[]string{PermissionUsersRead,PermissionUsersCreate,PermissionUsersUpdate,PermissionUsersDelete,PermissionUsersActivate,PermissionUsersDeactivate,PermissionRolesAssign}
    for _,role:=range roles{for _,permission:=range permissions{if HasPermission(role,permission){t.Fatalf("role %q unexpectedly has %q",role,permission)}}}
}
func TestStudentHasPortalAccess(t *testing.T) {\n    if !HasPermission(RoleStudent, PermissionStudentPortalRead) { t.Fatal("student should have student portal access") }\n    if HasPermission(RoleStudent, PermissionStudentsRead) { t.Fatal("student should not have administrative student-read permission") }\n}\n\nfunc TestTeacherCanManageAttendance(t *testing.T) {
    if !HasPermission(RoleTeacher,PermissionAttendanceRead){t.Fatal("teacher should read attendance")}
    if !HasPermission(RoleTeacher,PermissionAttendanceManage){t.Fatal("teacher should manage attendance")}
}
