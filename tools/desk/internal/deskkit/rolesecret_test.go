package deskkit

import (
	"strings"
	"testing"
)

// One spelling of the role-secret path variable, shared by the launcher and
// the dispatcher, and a strip that never returns nil.
// regression: #6065
func TestRoleSecretEnvHelpers(t *testing.T) {
	for svc, want := range map[string]string{"fake-svc": "ASSAY_ROLE_SECRET_FAKE_SVC_FILE", "db.read": "ASSAY_ROLE_SECRET_DB_READ_FILE", "x_1": "ASSAY_ROLE_SECRET_X_1_FILE"} {
		if got := RoleSecretEnvName(svc); got != want {
			t.Errorf("RoleSecretEnvName(%q) = %q, want %q", svc, got, want)
		}
		if !ValidRoleSecretService(svc) || !IsRoleSecretEnvName(RoleSecretEnvName(svc)) {
			t.Errorf("%q not recognised", svc)
		}
	}
	for _, bad := range []string{"", "Upper", "-lead", "has space", "a/b", "a=b"} {
		if ValidRoleSecretService(bad) {
			t.Errorf("service %q accepted", bad)
		}
	}
	if !IsRoleSecretEnvName("assay_role_secret_x_file") || IsRoleSecretEnvName("ASSAY_APP_PEM_FILE") {
		t.Error("name classification")
	}
	env := []string{"PATH=/bin", "ASSAY_ROLE_SECRET_FAKE_SVC_FILE=/run/secrets/role-fake-svc", "ASSAY_ROLE_SECRET_EMPTY_FILE=", "HOME=/h"}
	if !HoldsRoleSecret(env) || HoldsRoleSecret([]string{"PATH=/bin"}) || !HoldsRoleSecret([]string{"ASSAY_ROLE_SECRET_EMPTY_FILE="}) {
		t.Error("holding-window classification")
	}
	if got := strings.Join(StripRoleSecretEnv(env), ","); got != "PATH=/bin,HOME=/h" {
		t.Errorf("strip = %q", got)
	}
	if StripRoleSecretEnv(nil) == nil {
		t.Error("strip returned nil, which os/exec reads as inherit")
	}
	if !isSecretEnvName(RoleSecretEnvName("fake-svc")) {
		t.Error("the hook scrub does not cover the role-secret variable")
	}
}
