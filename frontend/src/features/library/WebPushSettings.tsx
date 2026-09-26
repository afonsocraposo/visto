import { useEffect, useState } from "react";
import { Alert, Button, Group, Text, Title } from "@mantine/core";

const supported = () =>
  "serviceWorker" in navigator && "PushManager" in window && "Notification" in window;
function decodeKey(value: string): Uint8Array<ArrayBuffer> {
  const raw = atob(
    value.replace(/-/g, "+").replace(/_/g, "/") + "=".repeat((4 - (value.length % 4)) % 4),
  );
  return Uint8Array.from(raw, (char) => char.charCodeAt(0));
}
export function WebPushSettings() {
  const [enabled, setEnabled] = useState(false);
  const [available, setAvailable] = useState<boolean | null>(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [registration, setRegistration] = useState<ServiceWorkerRegistration | null>(null);
  const [subscription, setSubscription] = useState<PushSubscription | null>(null);
  const [publicKey, setPublicKey] = useState("");
  const [permission, setPermission] = useState<NotificationPermission | null>(
    supported() ? Notification.permission : null,
  );
  useEffect(() => {
    if (!supported()) {
      setAvailable(false);
      return;
    }
    Promise.all([fetch("/api/v1/profile/web-push-key"), navigator.serviceWorker.ready])
      .then(async ([response, ready]) => {
        if (!response.ok) {
          setAvailable(false);
          return;
        }
        const { public_key } = await response.json();
        const current = await ready.pushManager.getSubscription();
        setPublicKey(public_key);
        setRegistration(ready);
        setSubscription(current);
        setAvailable(true);
        if (current) {
          const status = await fetch("/api/v1/profile/web-push-status", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ endpoint: current.endpoint }),
          });
          if (status.ok) setEnabled((await status.json()).enabled);
        }
      })
      .catch(() => setAvailable(false));
  }, []);
  const enable = async () => {
    if (!registration || !publicKey) return;
    setBusy(true);
    setMessage("");
    // Start subscribe directly from the tap. iOS requires this user gesture.
    try {
      const pending = subscription
        ? Promise.resolve(subscription)
        : registration.pushManager.subscribe({
            userVisibleOnly: true,
            applicationServerKey: decodeKey(publicKey),
          });
      const current = await pending;
      const response = await fetch("/api/v1/profile/web-push-subscription", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(current),
      });
      if (!response.ok) {
        if (!subscription) await current.unsubscribe();
        throw new Error("Could not save this device. Try again.");
      }
      setSubscription(current);
      setEnabled(true);
      setMessage("Notifications enabled on this device.");
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Could not enable notifications.");
    } finally {
      setPermission(Notification.permission);
      setBusy(false);
    }
  };
  const disable = async () => {
    setBusy(true);
    setMessage("");
    try {
      const subscription = await (
        await navigator.serviceWorker.ready
      ).pushManager.getSubscription();
      if (subscription) {
        const response = await fetch("/api/v1/profile/web-push-subscription", {
          method: "DELETE",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ endpoint: subscription.endpoint }),
        });
        if (!response.ok) throw new Error("Could not remove this device.");
        await subscription.unsubscribe();
      }
      setSubscription(null);
      setEnabled(false);
      setMessage("Notifications disabled on this device.");
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Could not disable notifications.");
    } finally {
      setBusy(false);
    }
  };
  const test = async () => {
    setBusy(true);
    setMessage("");
    try {
      const subscription = await (
        await navigator.serviceWorker.ready
      ).pushManager.getSubscription();
      if (!subscription) throw new Error("Enable notifications on this device first.");
      const response = await fetch("/api/v1/profile/web-push-test", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(subscription),
      });
      if (!response.ok) {
        const result = await response.json().catch(() => ({}));
        throw new Error(result.error || "Could not send test notification.");
      }
      setMessage("Test sent. Check this device for the notification.");
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Could not send test notification.");
    } finally {
      setBusy(false);
    }
  };
  return (
    <>
      <Title order={3} mt="xl">
        Browser notifications
      </Title>
      <Text size="sm" c="dimmed" mt="xs">
        Get episode alerts on this device, even when Visto is closed.
      </Text>
      <Text size="sm" mt="xs">
        {enabled ? "Enabled on this device" : "Off on this device"}
      </Text>
      {available === false && (
        <Text size="sm" c="dimmed" mt="xs">
          Browser notifications are unavailable here. On iPhone or iPad, add Visto to the Home
          Screen, then open it from there. Your server must also enable Web Push.
        </Text>
      )}
      {permission === "denied" && (
        <Text size="sm" c="dimmed" mt="xs">
          Notifications are blocked in this browser. Allow them in the site settings to enable
          alerts.
        </Text>
      )}
      {message && <Alert mt="md">{message}</Alert>}
      {available && (
        <Group mt="md">
          <Button
            loading={busy}
            disabled={!enabled && permission === "denied"}
            onClick={enabled ? disable : enable}
          >
            {enabled ? "Turn off on this device" : "Enable notifications on this device"}
          </Button>
          {enabled && (
            <Button variant="default" loading={busy} onClick={test}>
              Send test notification
            </Button>
          )}
        </Group>
      )}
    </>
  );
}
