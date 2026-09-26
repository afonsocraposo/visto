import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Alert, Button, Checkbox, Group, NumberInput, Paper, PasswordInput, Select, Stack, Text, TextInput, Title } from "@mantine/core";

type BackupSettings = {
  destination: "local" | "s3" | "both";
  interval_seconds: number;
  bucket: string;
  region: string;
  endpoint: string;
  access_key_id: string;
  path_style: boolean;
  prefix: string;
  max_keep: number;
  has_secret: boolean;
  encryption_available: boolean;
  last_success_at: string;
  last_error: string;
};

export function BackupPanel() {
  const queryClient = useQueryClient();
  const settings = useQuery({queryKey:["admin-backups"],queryFn:async()=>{const response=await fetch("/api/v1/admin/backups");if(!response.ok)throw new Error("Could not load backup settings.");return response.json() as Promise<BackupSettings>}});
  const [form,setForm]=useState<BackupSettings | null>(null);
  const [secret,setSecret]=useState("");
  const [message,setMessage]=useState("");
  useEffect(()=>{if(settings.data)setForm(settings.data)},[settings.data]);
  const save=useMutation({mutationFn:async()=>{const response=await fetch("/api/v1/admin/backups",{method:"PUT",headers:{"Content-Type":"application/json"},body:JSON.stringify({...form,secret_key:secret})});const result=await response.json();if(!response.ok)throw new Error(result.error || "Could not save backup settings.");return result as BackupSettings},onSuccess:async()=>{setSecret("");setMessage("Backup settings saved.");await queryClient.invalidateQueries({queryKey:["admin-backups"]})}});
  const action=useMutation({mutationFn:async(kind:"test"|"run")=>{const response=await fetch(`/api/v1/admin/backups/${kind}`,{method:"POST"});if(!response.ok){const result=await response.json().catch(()=>({}));throw new Error(result.error || `Could not ${kind} backup.`)}return kind},onSuccess:(kind)=>{setMessage(kind==="test"?"S3 connection works.":"Backup started. Refresh this page to see the result.")}});
  const update=<K extends keyof BackupSettings>(key:K,value:BackupSettings[K])=>setForm(current=>current?{...current,[key]:value}:current);
  if(settings.isPending)return <Text>Loading backup settings…</Text>;
  if(settings.isError||!form)return <Alert color="red">Could not load backup settings.</Alert>;
  const usesS3=form.destination!=="local";
  return <Paper withBorder p="md"><Stack>
    <Title order={3}>Backups</Title>
    <Text size="sm" c="dimmed">Automatic backups include the Visto database. Keep the encryption key outside your database backup.</Text>
    <Select label="Destination" data={[{value:"local",label:"Local"},{value:"s3",label:"S3"},{value:"both",label:"Local and S3"}]} value={form.destination} onChange={value=>{if(value)update("destination",value as BackupSettings["destination"])}} />
    <NumberInput label="Hours between backups" min={1} max={168} value={form.interval_seconds/3600} onChange={value=>update("interval_seconds",Number(value)*3600)} />
    {usesS3&&<>
      {!form.encryption_available&&<Alert color="yellow">Set VISTO_SECRET_ENCRYPTION_KEY on the server before saving S3 credentials.</Alert>}
      <TextInput required label="Bucket" value={form.bucket} onChange={event=>update("bucket",event.currentTarget.value)} />
      <TextInput required label="Region" value={form.region} onChange={event=>update("region",event.currentTarget.value)} />
      <TextInput label="S3 endpoint (optional)" placeholder="https://s3.example.com" value={form.endpoint} onChange={event=>update("endpoint",event.currentTarget.value)} />
      <Checkbox label="Use path-style addressing" checked={form.path_style} onChange={event=>update("path_style",event.currentTarget.checked)} />
      <TextInput required label="Access key ID" value={form.access_key_id} onChange={event=>update("access_key_id",event.currentTarget.value)} />
      <PasswordInput label="Secret access key" description={form.has_secret?"Leave blank to keep the saved key.":"Required for S3 backups."} value={secret} onChange={event=>setSecret(event.currentTarget.value)} />
      <TextInput label="Object prefix" value={form.prefix} onChange={event=>update("prefix",event.currentTarget.value)} />
      <NumberInput label="Scheduled S3 backups to keep" min={1} max={1000} value={form.max_keep} onChange={value=>update("max_keep",Number(value))} />
    </>}
    {(save.isError||action.isError)&&<Alert color="red">{save.error?.message||action.error?.message}</Alert>}
    {message&&<Alert color="green">{message}</Alert>}
    <Group><Button onClick={()=>{setMessage("");save.mutate()}} loading={save.isPending} disabled={usesS3&&!form.encryption_available}>Save settings</Button><Button variant="default" disabled={!usesS3||!form.has_secret} loading={action.isPending} onClick={()=>action.mutate("test")}>Test connection</Button><Button variant="default" loading={action.isPending} onClick={()=>action.mutate("run")}>Back up now</Button></Group>
    <Text size="sm">Last success: {form.last_success_at?new Date(form.last_success_at).toLocaleString():"None yet"}</Text>
    {form.last_error&&<Alert color="red">Last error: {form.last_error}</Alert>}
  </Stack></Paper>;
}
