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
)

var rolePermissions = map[string]map[string]bool{
    RoleSuperAdmin: {
        PermissionUsersRead: true, PermissionUsersCreate: true, PermissionUsersUpdate: true,
        PermissionUsersDelete: true, PermissionUsersActivate: true, PermissionUsersDeactivate: true,
        PermissionStudentsRead: true, PermissionStudentsCreate: true, PermissionStudentsUpdate: true, PermissionStudentsDelete: true,
        PermissionRolesAssign: true, PermissionAuditRead: true, PermissionAcademicRead: true, PermissionAcademicManage: true, PermissionClassesRead: true, PermissionClassesManage: true,
    },
    RoleSchoolAdmin: {
        PermissionUsersRead: true, PermissionUsersCreate: true, PermissionUsersUpdate: true,
        PermissionUsersActivate: true, PermissionUsersDeactivate: true,
        PermissionStudentsRead: true, PermissionStudentsCreate: true, PermissionStudentsUpdate: true, PermissionStudentsDelete: true, PermissionAcademicRead: true, PermissionAcademicManage: true,
    },
    RoleTeacher: {},
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
