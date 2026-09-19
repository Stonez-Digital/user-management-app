package authz

const (
    RoleSuperAdmin = "super_admin"
    RoleSchoolAdmin = "school_admin"
    RoleTeacher = "teacher"
    RoleStudent = "student"
    RoleParent = "parent"
    RoleAccountant = "accountant"
    RoleStaff = "staff"
)

const (
    PermissionUsersRead = "users.read"
    PermissionUsersCreate = "users.create"
    PermissionUsersUpdate = "users.update"
    PermissionUsersDelete = "users.delete"
    PermissionUsersActivate = "users.activate"
    PermissionUsersDeactivate = "users.deactivate"
    PermissionRolesAssign = "roles.assign"
    PermissionAuditRead = "audit.read"
    PermissionStudentsRead = "students.read"
    PermissionStudentsCreate = "students.create"
    PermissionStudentsUpdate = "students.update"
    PermissionStudentsDelete = "students.delete"
    PermissionAcademicRead = "academic.read"
    PermissionAcademicManage = "academic.manage"
    PermissionClassesRead = "classes.read"
    PermissionClassesManage = "classes.manage"
    PermissionSubjectsRead = "subjects.read"
    PermissionSubjectsManage = "subjects.manage"
    PermissionAssignmentsRead = "teacher_assignments.read"
    PermissionAssignmentsManage = "teacher_assignments.manage"
    PermissionEnrollmentRead = "student_enrollment.read"
    PermissionEnrollmentManage = "student_enrollment.manage"
    PermissionAttendanceRead = "attendance.read"
    PermissionAttendanceManage = "attendance.manage"
    PermissionAssessmentsRead = "assessments.read"
    PermissionAssessmentsManage = "assessments.manage"
    PermissionResultsRead = "results.read"
    PermissionResultsManage = "results.manage"
)

var rolePermissions = map[string]map[string]bool{
    RoleSuperAdmin: {
        PermissionUsersRead: true, PermissionUsersCreate: true, PermissionUsersUpdate: true,
        PermissionUsersDelete: true, PermissionUsersActivate: true, PermissionUsersDeactivate: true,
        PermissionStudentsRead: true, PermissionStudentsCreate: true, PermissionStudentsUpdate: true, PermissionStudentsDelete: true,
        PermissionRolesAssign: true, PermissionAuditRead: true, PermissionAcademicRead: true, PermissionAcademicManage: true, PermissionClassesRead: true, PermissionClassesManage: true, PermissionSubjectsRead: true, PermissionSubjectsManage: true, PermissionAssignmentsRead: true, PermissionAssignmentsManage: true, PermissionEnrollmentRead: true, PermissionEnrollmentManage: true, PermissionAttendanceRead: true, PermissionAttendanceManage: true, PermissionAssessmentsRead: true, PermissionAssessmentsManage: true, PermissionResultsRead: true, PermissionResultsManage: true,
    },
    RoleSchoolAdmin: {
        PermissionUsersRead: true, PermissionUsersCreate: true, PermissionUsersUpdate: true,
        PermissionUsersActivate: true, PermissionUsersDeactivate: true,
        PermissionStudentsRead: true, PermissionStudentsCreate: true, PermissionStudentsUpdate: true, PermissionStudentsDelete: true, PermissionAcademicRead: true, PermissionAcademicManage: true, PermissionSubjectsRead: true, PermissionSubjectsManage: true, PermissionAssignmentsRead: true, PermissionAssignmentsManage: true, PermissionEnrollmentRead: true, PermissionEnrollmentManage: true, PermissionAttendanceRead: true, PermissionAttendanceManage: true, PermissionAssessmentsRead: true, PermissionAssessmentsManage: true, PermissionResultsRead: true, PermissionResultsManage: true,
    },
    RoleTeacher: {
        PermissionAcademicRead: true, PermissionClassesRead: true, PermissionSubjectsRead: true,
        PermissionAssignmentsRead: true, PermissionEnrollmentRead: true,
        PermissionAttendanceRead: true, PermissionAttendanceManage: true,
        PermissionAssessmentsRead: true, PermissionAssessmentsManage: true,
        PermissionResultsRead: true, PermissionResultsManage: true,
    },
    RoleStudent: {},
    RoleParent: {},
    RoleAccountant: {},
    RoleStaff: {},
}

func IsValidRole(role string) bool {
    _, ok := rolePermissions[role]
    return ok
}

func HasPermission(role, permission string) bool {
    return rolePermissions[role][permission]
}

func PermissionsForRole(role string) []string {
    permissions := rolePermissions[role]
    result := make([]string, 0, len(permissions))
    for permission, allowed := range permissions {
        if allowed {
            result = append(result, permission)
        }
    }
    return result
}
