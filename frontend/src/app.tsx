import { FormEvent, useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Alert, AppShell, Button, Group, Loader, MantineProvider, Paper, PasswordInput, Select, Tabs, Text, TextInput, Title } from "@mantine/core";
import { IconCalendar, IconCompass, IconHome, IconLibrary, IconSearch } from "@tabler/icons-react";

type Tab = "watch" | "search" | "feed" | "library";
type Theme = "system" | "light" | "dark";
type User = { id: string; display_name: string };

export function App() {
  const session = useQuery({ queryKey: ["session"], queryFn: async (): Promise<User | null> => { const response = await fetch("/api/v1/me"); return response.ok ? response.json() : null; } });
  const [theme, setTheme] = useState<Theme>(() => (localStorage.getItem("visto-theme") as Theme) || "system");
  useEffect(() => { localStorage.setItem("visto-theme", theme); }, [theme]);
  return <MantineProvider defaultColorScheme={theme === "system" ? "auto" : theme}>{session.isPending ? <Group justify="center" mt="xl"><Loader /></Group> : session.data === null ? <Login /> : <Dashboard user={session.data!} theme={theme} setTheme={setTheme} />}</MantineProvider>;
}

function Login() {
  const queryClient = useQueryClient();
  const [username, setUsername] = useState(""); const [password, setPassword] = useState(""); const [error, setError] = useState("");
  const login = useMutation({ mutationFn: async () => { const response = await fetch("/api/v1/auth/login", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ username, password }) }); if (!response.ok) throw new Error("Invalid username or password."); return response.json() as Promise<User>; }, onSuccess: user => queryClient.setQueryData(["session"], user) });
  async function submit(event: FormEvent) { event.preventDefault(); setError(""); try { await login.mutateAsync(); } catch (reason) { setError(reason instanceof Error ? reason.message : "Could not sign in."); } }
  return <Paper withBorder radius="md" p="xl" maw={420} mx="auto" mt="xl"><Title order={1}>Welcome to Visto</Title><Text c="dimmed" mt="xs">Sign in to track what you watch.</Text><form onSubmit={submit}><TextInput required label="Username" value={username} onChange={event => setUsername(event.currentTarget.value)} mt="lg" /><PasswordInput required label="Password" value={password} onChange={event => setPassword(event.currentTarget.value)} mt="md" />{error && <Alert color="red" mt="md">{error}</Alert>}<Button type="submit" loading={login.isPending} fullWidth mt="lg">Sign in</Button></form></Paper>;
}

type SearchMedia = { tmdb_id: number; type: "movie" | "tv"; title: string; original_title: string; overview: string; release_date: string; poster_path: string; original_language: string };
type LibraryEntry = { item: { media_id: string; status: string; rating: number | null }; media: SearchMedia & { id: string } };
type FeedItem = { id: string; display_name: string; kind: "watch" | "rewatch" | "rating"; title: string; rating?: number; season_number?: number; episode_number?: number };

function Dashboard({ user, theme, setTheme }: { user: User; theme: Theme; setTheme: (theme: Theme) => void }) {
  const [tab, setTab] = useState<Tab>("watch"); const [view, setView] = useState("now");
  const nav = (value: Tab, label: string, Icon: typeof IconHome) => <Button variant={tab === value ? "light" : "subtle"} leftSection={<Icon size={18} />} onClick={() => setTab(value)}>{label}</Button>;
  return <AppShell header={{ height: 64 }} footer={{ height: 70 }} padding="md"><AppShell.Header><Group h="100%" px="md" justify="space-between"><Title order={2}>Visto</Title><Group><Text size="sm">Hi, {user.display_name}</Text><Select value={theme} onChange={value => setTheme((value || "system") as Theme)} data={[{ value: "system", label: "System" }, { value: "light", label: "Light" }, { value: "dark", label: "Dark" }]} w={110} /></Group></Group></AppShell.Header><AppShell.Main>{tab === "watch" && <><Tabs value={view} onChange={value => setView(value || "now")}><Tabs.List><Tabs.Tab value="now">Now</Tabs.Tab><Tabs.Tab value="calendar" leftSection={<IconCalendar size={16} />}>Calendar</Tabs.Tab></Tabs.List></Tabs><Empty title={view === "now" ? "Nothing to continue yet" : "No upcoming episodes"} /></>}{tab === "search" && <SearchPanel />}{tab === "feed" && <FeedPanel />}{tab === "library" && <LibraryPanel />}</AppShell.Main><AppShell.Footer><Group justify="space-around" h="100%">{nav("watch", "Watch", IconHome)}{nav("search", "Search", IconSearch)}{nav("feed", "Feed", IconCompass)}{nav("library", "Library", IconLibrary)}</Group></AppShell.Footer></AppShell>;
}
function SearchPanel() {
  const queryClient = useQueryClient(); const [query, setQuery] = useState("");
  const results = useQuery({ queryKey: ["search", query], enabled: query.trim().length > 1, queryFn: async () => { const response = await fetch(`/api/v1/search?q=${encodeURIComponent(query)}`); if (!response.ok) throw new Error(); return response.json() as Promise<SearchMedia[]>; } });
  const save = useMutation({ mutationFn: async (media: SearchMedia) => { const response = await fetch("/api/v1/library", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ media, status: "watchlist" }) }); if (!response.ok) throw new Error("Could not add this title."); }, onSuccess: () => queryClient.invalidateQueries({ queryKey: ["library"] }) });
  return <><Title order={1}>Search</Title><TextInput mt="md" label="Search TMDB" value={query} onChange={event => setQuery(event.currentTarget.value)} leftSection={<IconSearch size={16} />} />{results.isError && <Alert color="red" mt="md">Search is temporarily unavailable.</Alert>}{results.data?.map(item => <Paper key={`${item.type}-${item.tmdb_id}`} withBorder p="md" mt="sm"><Group justify="space-between" align="start"><div><Text fw={700}>{item.title}</Text><Text c="dimmed">{item.type === "tv" ? "TV show" : "Movie"}{item.release_date ? ` · ${item.release_date.slice(0, 4)}` : ""}</Text></div><Button size="xs" onClick={() => save.mutate(item)} loading={save.isPending}>Add</Button></Group></Paper>)}</>;
}

function LibraryPanel() {
  const library = useQuery({ queryKey: ["library"], queryFn: async () => { const response = await fetch("/api/v1/library"); if (!response.ok) throw new Error(); return response.json() as Promise<LibraryEntry[]>; } });
  if (library.isPending) return <Group justify="center" mt="xl"><Loader /></Group>;
  if (library.isError) return <Alert color="red" mt="md">Your library is temporarily unavailable.</Alert>;
  if (!library.data?.length) return <Empty title="Your library is empty" />;
  return <><Title order={1}>Library</Title>{library.data.map(entry => <Paper key={entry.item.media_id} withBorder p="md" mt="sm"><Text fw={700}>{entry.media.title}</Text><Text c="dimmed">{entry.item.status}{entry.item.rating ? ` · ${entry.item.rating}/5` : ""}</Text></Paper>)}</>;
}

function FeedPanel() {
  const feed = useQuery({ queryKey: ["feed"], queryFn: async () => { const response = await fetch("/api/v1/feed"); if (!response.ok) throw new Error(); return response.json() as Promise<{ items: FeedItem[] }>; } });
  if (feed.isPending) return <Group justify="center" mt="xl"><Loader /></Group>;
  if (feed.isError) return <Alert color="red" mt="md">The feed is temporarily unavailable.</Alert>;
  if (!feed.data?.items.length) return <Empty title="No shared activity yet" />;
  return <><Title order={1}>Feed</Title>{feed.data.items.map(item => <Paper key={item.id} withBorder p="md" mt="sm"><Text>{item.display_name} {item.kind === "rewatch" ? "rewatched" : item.kind === "rating" ? "rated" : "watched"} <Text component="span" fw={700}>{item.title}</Text>{item.kind === "rating" && item.rating ? ` · ${"★".repeat(item.rating)}` : item.season_number ? ` · S${String(item.season_number).padStart(2, "0")}E${String(item.episode_number).padStart(2, "0")}` : ""}</Text></Paper>)}</>;
}
function Empty({ title }: { title: string }) { return <Paper withBorder p="xl" mt="md" radius="md" ta="center"><Text fw={700}>{title}</Text></Paper>; }
