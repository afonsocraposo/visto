import { useEffect } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useForm } from "@mantine/form";
import { Alert, Button, Group, PasswordInput, Select, Text, TextInput } from "@mantine/core";
import { IconDownload, IconLogout } from "@tabler/icons-react";
import { useUserQueryKey } from "../auth/SessionContext";
import type { Theme, User } from "../../types";
import { PersonalTokensPanel } from "./PersonalTokensPanel";
import { ConnectedAppsPanel } from "./ConnectedAppsPanel";
import { PlexSyncPanel } from "./PlexSyncPanel";
import { WebPushSettings } from "./WebPushSettings";
import { ImportData } from "./ImportData";
import { showActionFeedback } from "../../lib/actionFeedback";
import { QueryError } from "../../components/QueryError";
import { SettingsIndex, SettingsSection, SettingsSubsection } from "./SettingsSection";

export function ProfilePanel({
  user,
  theme,
  onThemeChange,
  onSignOut,
  signingOut,
  signOutError,
}: {
  user: User;
  theme: Theme;
  onThemeChange: (theme: Theme) => void;
  onSignOut: () => void;
  signingOut: boolean;
  signOutError?: string;
}) {
  const queryClient = useQueryClient();
  const userQueryKey = useUserQueryKey();
  const form = useForm({
    initialValues: {
      visibility: "private",
      timezone: "UTC",
      pushoverAppToken: "",
      pushoverUserKey: "",
    },
  });
  const settings = useQuery({
    queryKey: userQueryKey("profile-settings"),
    queryFn: async () => {
      const response = await fetch("/api/v1/profile/activity-settings");
      if (!response.ok) throw new Error();
      return response.json() as Promise<{
        activity_visibility: string;
        timezone: string;
        pushover_available: boolean;
        pushover_enabled: boolean;
        has_pushover_app_token: boolean;
        has_pushover_user_key: boolean;
      }>;
    },
  });
  useEffect(() => {
    if (settings.data) {
      const saved = {
        visibility: settings.data.activity_visibility,
        timezone: settings.data.timezone,
      };
      form.setValues(saved);
      form.resetDirty({ ...form.getValues(), ...saved });
    }
  }, [settings.data]);
  const save = useMutation({
    mutationFn: async () => {
      const response = await fetch("/api/v1/profile/activity-settings", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          activity_visibility: form.values.visibility,
          timezone: form.values.timezone,
        }),
      });
      if (!response.ok) throw new Error("Could not save settings.");
    },
    onSuccess: async () => {
      form.resetDirty();
      showActionFeedback("Profile settings saved.");
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: userQueryKey("profile-settings") }),
        queryClient.invalidateQueries({ queryKey: userQueryKey("feed") }),
      ]);
    },
  });
  const savePushover = useMutation({
    mutationFn: async () => {
      const response = await fetch("/api/v1/profile/pushover-settings", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          enabled: settings.data?.pushover_enabled ?? false,
          app_token: form.values.pushoverAppToken,
          user_key: form.values.pushoverUserKey,
        }),
      });
      if (!response.ok) {
        const result = await response.json().catch(() => ({}));
        throw new Error(result.error || "Could not save Pushover settings.");
      }
    },
    onSuccess: async () => {
      form.setValues({ pushoverAppToken: "", pushoverUserKey: "" });
      showActionFeedback("Pushover settings saved.");
      await queryClient.invalidateQueries({ queryKey: userQueryKey("profile-settings") });
    },
  });
  const updatePushover = useMutation({
    mutationFn: async (enabled: boolean) => {
      const response = await fetch("/api/v1/profile/pushover-settings", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          enabled,
          app_token: form.values.pushoverAppToken,
          user_key: form.values.pushoverUserKey,
        }),
      });
      if (!response.ok) {
        const result = await response.json().catch(() => ({}));
        throw new Error(result.error || "Could not update notification settings.");
      }
    },
    onSuccess: async () => {
      form.setValues({ pushoverAppToken: "", pushoverUserKey: "" });
      showActionFeedback("Pushover settings updated.");
      await queryClient.invalidateQueries({ queryKey: userQueryKey("profile-settings") });
    },
  });
  const removePushoverKey = useMutation({
    mutationFn: async () => {
      const response = await fetch("/api/v1/profile/pushover-credentials", { method: "DELETE" });
      if (!response.ok) throw new Error("Could not remove Pushover credentials.");
    },
    onSuccess: async () => {
      showActionFeedback("Pushover credentials removed.");
      await queryClient.invalidateQueries({ queryKey: userQueryKey("profile-settings") });
    },
  });
  const testPushover = useMutation({
    mutationFn: async () => {
      const response = await fetch("/api/v1/profile/pushover-test", { method: "POST" });
      if (!response.ok) {
        const result = await response.json().catch(() => ({}));
        throw new Error(result.error || "Could not send Pushover test notification.");
      }
    },
    onSuccess: () => showActionFeedback("Test sent. Check Pushover on your device."),
  });
  const settingsUnavailable = settings.isPending || settings.isError;
  const profileDirty = form.isDirty("visibility") || form.isDirty("timezone");
  const hasPushoverCredentials = Boolean(
    settings.data?.has_pushover_app_token && settings.data?.has_pushover_user_key,
  );

  return (
    <div className="settings-layout">
      <SettingsIndex sections={sections} />
      <div className="settings-content">
        <SettingsSection
          id="profile"
          title="Profile"
          description="Choose who can see your activity and library, and set the calendar time zone."
        >
          {settings.isError && (
            <QueryError
              message="Profile settings are temporarily unavailable."
              onRetry={() => settings.refetch()}
            />
          )}
          <form
            onSubmit={(event) => {
              event.preventDefault();
              if (profileDirty) save.mutate();
            }}
          >
            <Select
              mt="md"
              label="Activity and library sharing"
              description="Visible to this instance lets other signed-in users see your library and recent activity."
              disabled={settingsUnavailable}
              allowDeselect={false}
              {...form.getInputProps("visibility")}
              data={[
                { value: "private", label: "Private" },
                { value: "instance", label: "Visible to this instance" },
              ]}
            />
            <TimeZoneInput
              disabled={settingsUnavailable}
              value={form.values.timezone}
              onChange={(value) => form.setFieldValue("timezone", value)}
            />
            {save.isError && (
              <Alert color="red" mt="md">
                {save.error.message}
              </Alert>
            )}
            <Button
              type="submit"
              mt="lg"
              disabled={settingsUnavailable || !profileDirty}
              loading={save.isPending}
            >
              Save settings
            </Button>
          </form>
        </SettingsSection>

        <SettingsSection
          id="appearance"
          title="Appearance"
          description="Choose how Visto looks on this device."
        >
          <Select
            mt="md"
            label="Color theme"
            value={theme}
            allowDeselect={false}
            onChange={(value) => onThemeChange((value || "system") as Theme)}
            data={[
              { value: "system", label: "System" },
              { value: "light", label: "Light" },
              { value: "dark", label: "Dark" },
            ]}
          />
        </SettingsSection>

        <SettingsSection
          id="notifications"
          title="Notifications"
          description="Get an alert when an unwatched regular episode airs for a show you are watching."
        >
          <WebPushSettings />
          <SettingsSubsection title="Pushover">
            {settings.isPending ? (
              <Text size="sm" c="dimmed" mt={4}>
                Checking Pushover settings…
              </Text>
            ) : settings.data?.pushover_available ? (
              <>
                <Text size="sm" mt={4} className="settings-status">
                  {settings.data.pushover_enabled
                    ? "Alerts are on."
                    : hasPushoverCredentials
                      ? "Alerts are off."
                      : "Add your Pushover credentials to turn alerts on."}
                </Text>
                <PasswordInput
                  mt="md"
                  label={
                    settings.data.has_pushover_app_token
                      ? "Replace your Pushover application token"
                      : "Your Pushover application token"
                  }
                  description="Create an application in your Pushover account to get this token."
                  autoComplete="off"
                  {...form.getInputProps("pushoverAppToken")}
                />
                <PasswordInput
                  mt="md"
                  label={
                    settings.data.has_pushover_user_key
                      ? "Replace your Pushover user key"
                      : "Your Pushover user key"
                  }
                  autoComplete="off"
                  {...form.getInputProps("pushoverUserKey")}
                />
                <Text size="xs" c="dimmed" mt={6}>
                  Both credentials are encrypted before saving and are never shown again.
                </Text>
                {(savePushover.isError ||
                  updatePushover.isError ||
                  removePushoverKey.isError ||
                  testPushover.isError) && (
                  <Alert color="red" mt="md">
                    {savePushover.error?.message ||
                      updatePushover.error?.message ||
                      removePushoverKey.error?.message ||
                      testPushover.error?.message}
                  </Alert>
                )}
                <Group mt="md" gap="sm">
                  {(form.values.pushoverAppToken || form.values.pushoverUserKey) && (
                    <Button loading={savePushover.isPending} onClick={() => savePushover.mutate()}>
                      Save credentials
                    </Button>
                  )}
                  {(settings.data.pushover_enabled || hasPushoverCredentials) && (
                    <Button
                      variant="default"
                      loading={updatePushover.isPending}
                      onClick={() => updatePushover.mutate(!settings.data!.pushover_enabled)}
                    >
                      {settings.data.pushover_enabled ? "Turn alerts off" : "Turn alerts on"}
                    </Button>
                  )}
                  {hasPushoverCredentials && (
                    <Button
                      variant="default"
                      loading={testPushover.isPending}
                      onClick={() => testPushover.mutate()}
                    >
                      Send test notification
                    </Button>
                  )}
                  {(settings.data.has_pushover_app_token ||
                    settings.data.has_pushover_user_key) && (
                    <Button
                      color="red"
                      variant="subtle"
                      loading={removePushoverKey.isPending}
                      onClick={() => {
                        if (
                          window.confirm(
                            "Remove your Pushover credentials? Alerts stop until you add them again.",
                          )
                        )
                          removePushoverKey.mutate();
                      }}
                    >
                      Remove credentials
                    </Button>
                  )}
                </Group>
              </>
            ) : (
              <Text size="sm" c="dimmed" mt={4}>
                This instance must enable encrypted storage for users’ notification credentials
                before Pushover can be used. Ask the administrator to configure
                VISTO_SECRET_ENCRYPTION_KEY.
              </Text>
            )}
          </SettingsSubsection>
        </SettingsSection>

        <PlexSyncPanel />

        <SettingsSection id="data" title="Your data">
          <SettingsSubsection title="Import your data" description="Choose an app to import from.">
            <Group mt="md">
              <ImportData />
            </Group>
          </SettingsSubsection>
          <SettingsSubsection
            title="Export your data"
            description="These downloads include only your library, ratings, and watch history."
          >
            <Group mt="md" gap="sm">
              <Button
                component="a"
                href="/api/v1/export/json"
                download="visto-export.json"
                variant="default"
                leftSection={<IconDownload size={16} />}
              >
                Download JSON
              </Button>
              <Button
                component="a"
                href="/api/v1/export/csv"
                download="visto-export.csv"
                variant="default"
                leftSection={<IconDownload size={16} />}
              >
                Download CSV
              </Button>
            </Group>
          </SettingsSubsection>
        </SettingsSection>

        <PersonalTokensPanel />
        <ConnectedAppsPanel />

        <SettingsSection id="account" title="Account">
          <div className="settings-account">
            <div className="settings-account-identity">
              {user.name && <Text fw={650}>{user.name}</Text>}
              <Text size="sm" c="dimmed" className="settings-account-email">
                {user.email}
              </Text>
            </div>
            <Button
              variant="default"
              leftSection={<IconLogout size={16} />}
              loading={signingOut}
              onClick={onSignOut}
            >
              Sign out
            </Button>
          </div>
          {signOutError && (
            <Alert color="red" mt="md">
              {signOutError}
            </Alert>
          )}
          <Text size="xs" c="dimmed" mt="lg" className="settings-version">
            Visto {__APP_VERSION__}
          </Text>
        </SettingsSection>
      </div>
    </div>
  );
}

const sections = [
  { id: "profile", label: "Profile" },
  { id: "appearance", label: "Appearance" },
  { id: "notifications", label: "Notifications" },
  { id: "plex", label: "Plex watch sync" },
  { id: "data", label: "Your data" },
  { id: "tokens", label: "API tokens" },
  { id: "apps", label: "Connected apps" },
  { id: "account", label: "Account" },
];

const timeZones = (() => {
  try {
    return Intl.supportedValuesOf("timeZone");
  } catch {
    return [];
  }
})();

function deviceTimeZone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone;
  } catch {
    return "";
  }
}

/** Searchable list of IANA zones; falls back to free text where the browser cannot list them. */
function TimeZoneInput({
  value,
  onChange,
  disabled,
}: {
  value: string;
  onChange: (value: string) => void;
  disabled: boolean;
}) {
  const device = deviceTimeZone();
  const suggestion =
    !disabled && device && device !== value ? (
      <div>
        <Button
          variant="subtle"
          color="gray"
          size="compact-sm"
          mt={6}
          className="settings-inline-action"
          onClick={() => onChange(device)}
        >
          Use this device’s time zone ({device.replace(/_/g, " ")})
        </Button>
      </div>
    ) : null;

  if (!timeZones.length) {
    return (
      <>
        <TextInput
          mt="md"
          label="Time zone"
          description="Use a time zone such as Europe/Lisbon or America/New_York."
          disabled={disabled}
          value={value}
          onChange={(event) => onChange(event.currentTarget.value)}
        />
        {suggestion}
      </>
    );
  }
  const data = Array.from(new Set(["UTC", ...timeZones, value].filter(Boolean))).map((zone) => ({
    value: zone,
    label: zone.replace(/_/g, " "),
  }));
  return (
    <>
      <Select
        mt="md"
        label="Time zone"
        description="Search for a city, such as Lisbon or New York."
        searchable
        allowDeselect={false}
        limit={60}
        nothingFoundMessage="No matching time zone"
        disabled={disabled}
        value={value}
        onChange={(next) => next && onChange(next)}
        data={data}
      />
      {suggestion}
    </>
  );
}
