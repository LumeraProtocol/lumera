package main

import "testing"

func TestEnvWithoutDesktopBusForcesTestKeyringBackend(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path=/run/user/1000/bus")
	t.Setenv("DISPLAY", ":1")
	t.Setenv("WAYLAND_DISPLAY", "wayland-1")
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	t.Setenv("LUMERA_KEYRING_BACKEND", "secret-service")
	t.Setenv("LUMERA_TEST_KEEP", "yes")

	env := envWithoutDesktopBus()
	for _, kv := range env {
		if kv == "DBUS_SESSION_BUS_ADDRESS=unix:path=/tmp/lumera-no-dbus-session-bus" {
			continue
		}
		if isDesktopSessionEnv(kv) {
			t.Fatalf("envWithoutDesktopBus kept desktop session env: %q", kv)
		}
	}
	if !containsEnvValue(env, "DBUS_SESSION_BUS_ADDRESS=unix:path=/tmp/lumera-no-dbus-session-bus") {
		t.Fatalf("envWithoutDesktopBus did not disable D-Bus autolaunch: %v", env)
	}
	if !containsEnvValue(env, "DISABLE_KWALLET=1") {
		t.Fatalf("envWithoutDesktopBus did not disable KWallet probing: %v", env)
	}
	if !containsEnvValue(env, "LUMERA_TEST_KEEP=yes") {
		t.Fatalf("envWithoutDesktopBus dropped unrelated env var: %v", env)
	}
	if !containsEnvValue(env, "LUMERA_KEYRING_BACKEND=test") {
		t.Fatalf("envWithoutDesktopBus did not force test keyring backend: %v", env)
	}
	if containsEnvValue(env, "LUMERA_KEYRING_BACKEND=secret-service") {
		t.Fatalf("envWithoutDesktopBus kept parent keyring backend: %v", env)
	}
}

func containsEnvValue(env []string, want string) bool {
	for _, kv := range env {
		if kv == want {
			return true
		}
	}
	return false
}
