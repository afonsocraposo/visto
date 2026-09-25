import { useEffect, useRef } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Alert, Button, Center, Loader, Stack, Text, Title } from "@mantine/core";
import { clearSignedInCache, endCurrentSession } from "./logout";

export function LogoutPage() {
  const queryClient = useQueryClient();
  const started = useRef(false);
  const logout = useMutation({
    mutationFn: endCurrentSession,
    onSuccess: () => {
      clearSignedInCache(queryClient);
      window.location.replace("/");
    },
  });

  useEffect(() => {
    if (started.current) return;
    started.current = true;
    logout.mutate();
  }, [logout.mutate]);

  return (
    <Center mih="60vh" p="md">
      <Stack align="center" maw={420} w="100%">
        {logout.isError ? (
          <>
            <Title order={2}>Could not sign out</Title>
            <Alert color="red" w="100%">
              {logout.error.message}
            </Alert>
            <Button onClick={() => logout.mutate()}>Try again</Button>
          </>
        ) : (
          <>
            <Loader size="sm" />
            <Text c="dimmed">Signing out…</Text>
          </>
        )}
      </Stack>
    </Center>
  );
}
