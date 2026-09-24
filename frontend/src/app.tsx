import { FormEvent, useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Alert, AppShell, Button, Group, Loader, MantineProvider, Modal, Paper, PasswordInput, Select, Tabs, Text, TextInput, Title } from "@mantine/core";
import { IconCalendar, IconCompass, IconHome, IconLibrary, IconSearch } from "@tabler/icons-react";

type Tab = "watch" | "search" | "feed" | "library";
type Theme = "system" | "light" | "dark";
type User = { id: string; username: string; display_name: string; role: "admin" | "user" };

export function App() {
  const session = useQuery({ queryKey: ["session"], queryFn: async (): Promise<User | null> => { const response = await fetch("/api/v1/me"); return response.ok ? response.json() : null; } });
  const [theme, setTheme] = useState<Theme>(() => (localStorage.getItem("visto-theme") as Theme) || "system");
  useEffect(() => { localStorage.setItem("visto-theme", theme); }, [theme]);
  return <MantineProvider defaultColorScheme={theme === "system" ? "auto" : theme}>{session.isPending ? <Group justify="center" mt="xl"><Loader /></Group> : session.data === null ? <Login /> : <Dashboard user={session.data!} theme={theme} setTheme={setTheme} />}</MantineProvider>;
}

function Login() {
  const queryClient = useQueryClient();
  const setup = useQuery({ queryKey: ["auth-status"], queryFn: async () => { const response = await fetch("/api/v1/auth/status"); if (!response.ok) throw new Error(); return response.json() as Promise<{ bootstrap_available: boolean }>; } });
  const [username, setUsername] = useState(""); const [password, setPassword] = useState(""); const [error, setError] = useState("");
  const login = useMutation({ mutationFn: async () => { const response = await fetch("/api/v1/auth/login", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ username, password }) }); if (!response.ok) throw new Error("Invalid username or password."); return response.json() as Promise<User>; }, onSuccess: user => queryClient.setQueryData(["session"], user) });
  async function submit(event: FormEvent) { event.preventDefault(); setError(""); try { await login.mutateAsync(); } catch (reason) { setError(reason instanceof Error ? reason.message : "Could not sign in."); } }
  if (setup.isPending) return <Group justify="center" mt="xl"><Loader /></Group>;
  if (setup.data?.bootstrap_available) return <BootstrapPanel />;
  return <Paper withBorder radius="md" p="xl" maw={420} mx="auto" mt="xl"><Title order={1}>Welcome to Visto</Title><Text c="dimmed" mt="xs">Sign in to track what you watch.</Text><form onSubmit={submit}><TextInput required label="Username" value={username} onChange={event => setUsername(event.currentTarget.value)} mt="lg" /><PasswordInput required label="Password" value={password} onChange={event => setPassword(event.currentTarget.value)} mt="md" />{error && <Alert color="red" mt="md">{error}</Alert>}<Button type="submit" loading={login.isPending} fullWidth mt="lg">Sign in</Button></form></Paper>;
}

function BootstrapPanel() {
  const queryClient=useQueryClient();const [username,setUsername]=useState("");const [displayName,setDisplayName]=useState("");const [password,setPassword]=useState("");const [error,setError]=useState("");
  const create=useMutation({mutationFn:async()=>{const body=JSON.stringify({username,display_name:displayName,password});const bootstrap=await fetch("/api/v1/auth/bootstrap",{method:"POST",headers:{"Content-Type":"application/json"},body});if(!bootstrap.ok){const result=await bootstrap.json().catch(()=>({}));throw new Error(result.error||"Could not create the administrator.")};const login=await fetch("/api/v1/auth/login",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({username,password})});if(!login.ok)throw new Error("Administrator created, but automatic sign in failed. Please sign in.");return login.json() as Promise<User>},onSuccess:user=>queryClient.setQueryData(["session"],user)});
  async function submit(event:FormEvent){event.preventDefault();setError("");try{await create.mutateAsync()}catch(reason){setError(reason instanceof Error?reason.message:"Could not finish setup.")}}
  return <Paper withBorder radius="md" p="xl" maw={440} mx="auto" mt="xl"><Title order={1}>Set up Visto</Title><Text c="dimmed" mt="xs">Create the first administrator for this instance.</Text><form onSubmit={submit}><TextInput required minLength={3} maxLength={32} label="Username" value={username} onChange={event=>setUsername(event.currentTarget.value)} mt="lg" /><TextInput required maxLength={80} label="Your name" value={displayName} onChange={event=>setDisplayName(event.currentTarget.value)} mt="md" /><PasswordInput required minLength={12} label="Password" value={password} onChange={event=>setPassword(event.currentTarget.value)} mt="md" />{error&&<Alert color="red" mt="md">{error}</Alert>}<Button type="submit" loading={create.isPending} fullWidth mt="lg">Create administrator</Button></form></Paper>;
}

type SearchMedia = { tmdb_id: number; type: "movie" | "tv"; title: string; original_title: string; overview: string; release_date: string; poster_path: string; original_language: string };
type LibraryEntry = { item: { media_id: string; status: string; rating: number | null }; media: SearchMedia & { id: string } };
type FeedItem = { id: string; display_name: string; kind: "watch" | "rewatch" | "rating" | "bulk_watch"; title: string; rating?: number; count?: number; season_number?: number; episode_number?: number };
type Episode = { id: string; season_number: number; episode_number: number; air_date: string | null };
type ContinueEntry = { show_id: string; title: string; poster_path: string; kind: "continue" | "start"; next_episode?: Episode; missing_prior_episodes?: Episode[] };
type CalendarEntry = { show_id: string; title: string; episode: Episode };

function Dashboard({ user, theme, setTheme }: { user: User; theme: Theme; setTheme: (theme: Theme) => void }) {
  const [tab, setTab] = useState<Tab>("watch"); const [view, setView] = useState("now");
  const [online,setOnline]=useState(()=>navigator.onLine);
  useEffect(()=>{const onlineHandler=()=>setOnline(true);const offlineHandler=()=>setOnline(false);window.addEventListener("online",onlineHandler);window.addEventListener("offline",offlineHandler);return()=>{window.removeEventListener("online",onlineHandler);window.removeEventListener("offline",offlineHandler)}},[]);
  const nav = (value: Tab, label: string, Icon: typeof IconHome) => <Button variant={tab === value ? "light" : "subtle"} leftSection={<Icon size={18} />} onClick={() => setTab(value)}>{label}</Button>;
  return <AppShell header={{ height: 64 }} footer={{ height: 70 }} padding="md"><AppShell.Header><Group h="100%" px="md" justify="space-between"><Title order={2}>Visto</Title><Group><Text size="sm">Hi, {user.display_name}</Text><Select value={theme} onChange={value => setTheme((value || "system") as Theme)} data={[{ value: "system", label: "System" }, { value: "light", label: "Light" }, { value: "dark", label: "Dark" }]} w={110} /></Group></Group></AppShell.Header><AppShell.Main>{!online&&<Alert color="yellow" mb="md">You are offline. Saved information may be out of date, and changes need a connection.</Alert>}{tab === "watch" && <><Tabs value={view} onChange={value => setView(value || "now")}><Tabs.List><Tabs.Tab value="now">Now</Tabs.Tab><Tabs.Tab value="calendar" leftSection={<IconCalendar size={16} />}>Calendar</Tabs.Tab></Tabs.List></Tabs>{view === "now" ? <WatchNow /> : <WatchCalendar />}</>}{tab === "search" && <SearchPanel />}{tab === "feed" && <FeedPanel />}{tab === "library" && <><LibraryPanel /><ProfilePanel user={user} /></>}</AppShell.Main><AppShell.Footer><Group justify="space-around" h="100%">{nav("watch", "Watch", IconHome)}{nav("search", "Search", IconSearch)}{nav("feed", "Feed", IconCompass)}{nav("library", "Library", IconLibrary)}</Group></AppShell.Footer></AppShell>;
}
function SearchPanel() {
  const queryClient = useQueryClient(); const [query, setQuery] = useState("");
  const library = useQuery({ queryKey: ["library"], queryFn: async () => { const response = await fetch("/api/v1/library"); if (!response.ok) throw new Error(); return response.json() as Promise<LibraryEntry[]>; } });
  const results = useQuery({ queryKey: ["search", query], enabled: query.trim().length > 1, queryFn: async () => { const response = await fetch(`/api/v1/search?q=${encodeURIComponent(query)}`); if (!response.ok) throw new Error(); return response.json() as Promise<SearchMedia[]>; } });
  const save = useMutation({ mutationFn: async (media: SearchMedia) => { const response = await fetch("/api/v1/library", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ media, status: "watchlist" }) }); if (!response.ok) throw new Error("Could not add this title."); }, onSuccess: () => queryClient.invalidateQueries({ queryKey: ["library"] }) });
  const libraryIDs=new Set(library.data?.map(entry=>entry.item.media_id)??[]);
  return <><Title order={1}>Search</Title><TextInput mt="md" label="Search TMDB" value={query} onChange={event => setQuery(event.currentTarget.value)} leftSection={<IconSearch size={16} />} />{results.isError && <Alert color="red" mt="md">Search is temporarily unavailable.</Alert>}{results.data?.map(item => {const mediaID=`${item.type}:${item.tmdb_id}`;const saved=libraryIDs.has(mediaID);return <Paper key={`${item.type}-${item.tmdb_id}`} withBorder p="md" mt="sm"><Group justify="space-between" align="start"><div><Text fw={700}>{item.title}</Text><Text c="dimmed">{item.type === "tv" ? "TV show" : "Movie"}{item.release_date ? ` · ${item.release_date.slice(0, 4)}` : ""}</Text></div><Button size="xs" disabled={saved} onClick={() => save.mutate(item)} loading={save.isPending}>{saved?"In library":"Add"}</Button></Group></Paper>})}</>;
}

function LibraryPanel() {
  const library = useQuery({ queryKey: ["library"], queryFn: async () => { const response = await fetch("/api/v1/library"); if (!response.ok) throw new Error(); return response.json() as Promise<LibraryEntry[]>; } });
  if (library.isPending) return <Group justify="center" mt="xl"><Loader /></Group>;
  if (library.isError) return <Alert color="red" mt="md">Your library is temporarily unavailable.</Alert>;
  if (!library.data?.length) return <Empty title="Your library is empty" />;
  return <><Title order={1}>Library</Title>{library.data.map(entry => <LibraryCard key={entry.item.media_id} entry={entry} />)}</>;
}

function LibraryCard({entry}:{entry:LibraryEntry}) {
  const queryClient=useQueryClient();
  const update=useMutation({mutationFn:async({status,rating}:{status:string;rating:number|null})=>{const response=await fetch(`/api/v1/library/${encodeURIComponent(entry.item.media_id)}`,{method:"PATCH",headers:{"Content-Type":"application/json"},body:JSON.stringify({status,rating})});if(!response.ok)throw new Error("Could not update this title.")},onSuccess:async()=>{await queryClient.invalidateQueries({queryKey:["library"]});await queryClient.invalidateQueries({queryKey:["continue"]});await queryClient.invalidateQueries({queryKey:["calendar"]});await queryClient.invalidateQueries({queryKey:["feed"]})}});
  const setStatus=(status:string|null)=>{if(status)update.mutate({status,rating:entry.item.rating})};
  const setRating=(value:string|null)=>update.mutate({status:entry.item.status,rating:value&&value!=="none"?Number(value):null});
  return <Paper withBorder p="md" mt="sm"><Group justify="space-between"><Text fw={700}>{entry.media.title}</Text><Text size="sm" c="dimmed">{entry.media.type==="tv"?"TV show":"Movie"}</Text></Group><Group mt="sm" grow><Select aria-label={`Status for ${entry.media.title}`} value={entry.item.status} onChange={setStatus} data={[{value:"watchlist",label:"Watchlist"},{value:"watching",label:"Watching"},{value:"paused",label:"Paused"},{value:"dropped",label:"Dropped"}]} /><Select aria-label={`Rating for ${entry.media.title}`} value={entry.item.rating?String(entry.item.rating):"none"} onChange={setRating} data={[{value:"none",label:"Not rated"},...[1,2,3,4,5].map(value=>({value:String(value),label:`${value} / 5 stars`}))]} /></Group>{update.isError&&<Alert color="red" mt="sm">{update.error.message}</Alert>}</Paper>;
}

function FeedPanel() {
  const feed = useQuery({ queryKey: ["feed"], queryFn: async () => { const response = await fetch("/api/v1/feed"); if (!response.ok) throw new Error(); return response.json() as Promise<{ items: FeedItem[] }>; } });
  if (feed.isPending) return <Group justify="center" mt="xl"><Loader /></Group>;
  if (feed.isError) return <Alert color="red" mt="md">The feed is temporarily unavailable.</Alert>;
  if (!feed.data?.items.length) return <Empty title="No shared activity yet" />;
  return <><Title order={1}>Feed</Title>{feed.data.items.map(item => <Paper key={item.id} withBorder p="md" mt="sm"><Text>{item.display_name} {item.kind === "rewatch" ? "rewatched" : item.kind === "rating" ? "rated" : item.kind === "bulk_watch" ? `marked ${item.count} episodes of` : "watched"} <Text component="span" fw={700}>{item.title}</Text>{item.kind === "rating" && item.rating ? ` · ${"★".repeat(item.rating)}` : item.season_number ? ` · S${String(item.season_number).padStart(2, "0")}E${String(item.episode_number).padStart(2, "0")}` : ""}</Text></Paper>)}</>;
}

function WatchNow() {
  const queryClient = useQueryClient();
  const [confirmation, setConfirmation] = useState<ContinueEntry | null>(null);
  const entries = useQuery({ queryKey: ["continue"], queryFn: async () => { const response = await fetch("/api/v1/continue-watching"); if (!response.ok) throw new Error(); return response.json() as Promise<ContinueEntry[]>; } });
  const markWatched = useMutation({ mutationFn: async ({ episodeIDs, bulk }: { episodeIDs: string[]; bulk: boolean }) => { const response = await fetch(bulk ? "/api/v1/plays/bulk" : "/api/v1/plays", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(bulk ? { episode_ids: episodeIDs } : { episode_id: episodeIDs[0] }) }); if (!response.ok) throw new Error("Could not mark episode watched."); }, onSuccess: async () => { setConfirmation(null); await queryClient.invalidateQueries({ queryKey: ["continue"] }); await queryClient.invalidateQueries({ queryKey: ["calendar"] }); await queryClient.invalidateQueries({ queryKey: ["feed"] }); await queryClient.invalidateQueries({ queryKey: ["library"] }); } });
  if (entries.isPending) return <Group justify="center" mt="xl"><Loader /></Group>;
  if (entries.isError) return <Alert color="red" mt="md">Watch data is temporarily unavailable.</Alert>;
  if (!entries.data?.length) return <Empty title="Nothing to continue yet" />;
  return <><Modal opened={confirmation !== null} onClose={() => setConfirmation(null)} title="Skipped episodes" centered><Text mb="md">You have {confirmation?.missing_prior_episodes?.length} earlier unplayed episodes of {confirmation?.title}. How would you like to continue?</Text><Group justify="flex-end"><Button variant="default" onClick={() => confirmation?.next_episode && markWatched.mutate({ episodeIDs: [confirmation.next_episode.id], bulk: false })} loading={markWatched.isPending}>Only this episode</Button><Button onClick={() => confirmation?.next_episode && markWatched.mutate({ episodeIDs: [...(confirmation.missing_prior_episodes || []).map(episode => episode.id), confirmation.next_episode.id], bulk: true })} loading={markWatched.isPending}>Mark all as watched</Button></Group></Modal>{entries.data.map(entry => <Paper key={entry.show_id} withBorder p="md" mt="md"><Group justify="space-between"><div><Text fw={700}>{entry.title}</Text><Text c="dimmed">{entry.kind === "start" ? "Ready to start" : "Continue watching"}{entry.next_episode ? ` · S${String(entry.next_episode.season_number).padStart(2, "0")}E${String(entry.next_episode.episode_number).padStart(2, "0")}` : ""}</Text></div>{entry.next_episode && <Button size="xs" loading={markWatched.isPending} onClick={() => (entry.missing_prior_episodes?.length ? setConfirmation(entry) : markWatched.mutate({ episodeIDs: [entry.next_episode!.id], bulk: false }))}>Watched</Button>}</Group></Paper>)}</>;
}

function WatchCalendar() {
  const calendar = useQuery({ queryKey: ["calendar"], queryFn: async () => { const response = await fetch("/api/v1/calendar"); if (!response.ok) throw new Error(); return response.json() as Promise<CalendarEntry[]>; } });
  if (calendar.isPending) return <Group justify="center" mt="xl"><Loader /></Group>;
  if (calendar.isError) return <Alert color="red" mt="md">Calendar is temporarily unavailable.</Alert>;
  if (!calendar.data?.length) return <Empty title="No upcoming episodes" />;
  return <>{calendar.data.map(item => <Paper key={item.episode.id} withBorder p="md" mt="sm"><Text fw={700}>{item.title}</Text><Text>{`S${String(item.episode.season_number).padStart(2, "0")}E${String(item.episode.episode_number).padStart(2, "0")}`}</Text><Text c="dimmed">{item.episode.air_date ? new Date(`${item.episode.air_date}T00:00:00`).toLocaleDateString() : "Date not announced"}</Text></Paper>)}</>;
}

function ProfilePanel({user}:{user:User}) {
  const queryClient = useQueryClient(); const [visibility,setVisibility]=useState("private"); const [timezone,setTimezone]=useState("UTC");
  const settings=useQuery({queryKey:["profile-settings"],queryFn:async()=>{const response=await fetch("/api/v1/profile/activity-settings");if(!response.ok)throw new Error();return response.json() as Promise<{activity_visibility:string;timezone:string}>}});
  useEffect(()=>{if(settings.data){setVisibility(settings.data.activity_visibility);setTimezone(settings.data.timezone)}},[settings.data]);
  const save=useMutation({mutationFn:async()=>{const response=await fetch("/api/v1/profile/activity-settings",{method:"PATCH",headers:{"Content-Type":"application/json"},body:JSON.stringify({activity_visibility:visibility,timezone})});if(!response.ok)throw new Error("Could not save settings.")},onSuccess:()=>queryClient.invalidateQueries({queryKey:["profile-settings"]})});
  return <><Paper withBorder p="md" mt="lg"><Title order={2}>Profile</Title><Text size="sm" c="dimmed" mt="xs">Choose who can see your activity and which time zone the calendar uses.</Text>{settings.isError&&<Alert color="red" mt="md">Profile settings are temporarily unavailable.</Alert>}<Select mt="md" label="Activity feed" value={visibility} onChange={value=>setVisibility(value||"private")} data={[{value:"private",label:"Private"},{value:"instance",label:"Visible to this instance"}]} /><TextInput mt="md" label="Time zone" description="Use a time zone such as Europe/Lisbon or America/New_York." value={timezone} onChange={event=>setTimezone(event.currentTarget.value)} />{save.isError&&<Alert color="red" mt="md">{save.error.message}</Alert>}<Button mt="md" loading={save.isPending} onClick={()=>save.mutate()}>Save settings</Button></Paper>{user.role==="admin"&&<CreateUserPanel />}</>;
}

function CreateUserPanel(){
  const [username,setUsername]=useState("");const [displayName,setDisplayName]=useState("");const [password,setPassword]=useState("");const [created,setCreated]=useState("");
  const create=useMutation({mutationFn:async()=>{const response=await fetch("/api/v1/users",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({username,display_name:displayName,password})});if(!response.ok){const result=await response.json().catch(()=>({}));throw new Error(result.error||"Could not create this account.")}return response.json() as Promise<User>},onSuccess:user=>{setCreated(user.display_name);setUsername("");setDisplayName("");setPassword("")}});
  async function submit(event:FormEvent){event.preventDefault();setCreated("");await create.mutateAsync()}
  return <Paper withBorder p="md" mt="lg"><Title order={2}>Add a family member</Title><Text size="sm" c="dimmed" mt="xs">Each person gets a separate library and watch history.</Text><form onSubmit={event=>{void submit(event).catch(()=>{})}}><TextInput required minLength={3} maxLength={32} label="Username" value={username} onChange={event=>setUsername(event.currentTarget.value)} mt="md" /><TextInput required maxLength={80} label="Name" value={displayName} onChange={event=>setDisplayName(event.currentTarget.value)} mt="md" /><PasswordInput required minLength={12} label="Temporary password" value={password} onChange={event=>setPassword(event.currentTarget.value)} mt="md" />{create.isError&&<Alert color="red" mt="md">{create.error.message}</Alert>}{created&&<Alert color="green" mt="md">Account created for {created}.</Alert>}<Button type="submit" loading={create.isPending} mt="md">Create account</Button></form></Paper>;
}
function Empty({ title }: { title: string }) { return <Paper withBorder p="xl" mt="md" radius="md" ta="center"><Text fw={700}>{title}</Text></Paper>; }
