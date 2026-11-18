package authz

import "testing"

func TestProductionRoleMatrixCoversAllSchoolRoles(t *testing.T) {
	roles := []string{
		RoleSuperAdmin,
		RoleSchoolAdmin,
		RoleTeacher,
		RoleStudent,
		RoleParent,
		RoleAccountant,
		RoleStaff,
	}

	for _, role := range roles {
		if !IsValidRole(role) {
			t.Fatalf("expected %q to be a valid role", role)
		}
		if len(PermissionsForRole(role)) == 0 {
			t.Fatalf("expected %q to have permissions", role)
		}
	}

	required := map[string][]string{
		RoleSuperAdmin: {
			PermissionRolesAssign,
			PermissionAuditRead,
			PermissionAcademicManage,
			PermissionFeesManage,
			PermissionParentPortalManage,
		},
		RoleSchoolAdmin: {
			PermissionUsersCreate,
			PermissionStudentsCreate,
			PermissionAcademicManage,
			PermissionFeesManage,
			PermissionParentPortalManage,
		},
		RoleTeacher: {
			PermissionTeacherWorkspaceRead,
			PermissionAttendanceManage,
			PermissionAssessmentsManage,
			PermissionResultsManage,
		},
		RoleStudent: {
			PermissionStudentPortalRead,
			PermissionTimetableRead,
		},
		RoleParent: {
			PermissionParentPortalRead,
			PermissionNotificationsRead,
		},
		RoleAccountant: {
			PermissionFeesManage,
			PermissionInvoicesManage,
			PermissionPaymentsManage,
		},
		RoleStaff: {
			PermissionNotificationsRead,
		},
	}

	for role, permissions := range required {
		for _, permission := range permissions {
			if !HasPermission(role, permission) {
				t.Fatalf("expected role %q to have permission %q", role, permission)
			}
		}
	}

	forbidden := map[string][]string{
		RoleTeacher: {
			PermissionRolesAssign,
			PermissionUsersCreate,
			PermissionFeesManage,
			PermissionParentPortalManage,
		},
		RoleStudent: {
			PermissionAcademicManage,
			PermissionFeesManage,
			PermissionResultsManage,
		},
		RoleParent: {
			PermissionAcademicManage,
			PermissionFeesManage,
			PermissionStudentPortalRead,
		},
		RoleAccountant: {
			PermissionRolesAssign,
			PermissionAcademicManage,
			PermissionStudentsCreate,
		},
		RoleStaff: {
			PermissionUsersCreate,
			PermissionAcademicManage,
			PermissionFeesManage,
		},
	}

	for role, permissions := range forbidden {
		for _, permission := range permissions {
			if HasPermission(role, permission) {
				t.Fatalf("expected role %q not to have permission %q", role, permission)
			}
		}
	}
}
