import { FormEvent, useEffect, useState } from "react";
import { Alert, AppShell, Button, Group, Loader, MantineProvider, Paper, PasswordInput, Select, Tabs, Text, TextInput, Title } from "@mantine/core";
import { IconCalendar, IconCompass, IconHome, IconLibrary, IconSearch } from "@tabler/icons-react";

type Tab = "watch" | "search" | "feed" | "library";
type Theme = "system" | "light" | "dark";
type User = { id: string; display_name: string };

export function App() {
  const [user, setUser] = useState<User | null | undefined>(undefined);
  const [theme, setTheme] = useState<Theme>(() => (localStorage.getItem("visto-theme") as Theme) || "system");
  useEffect(() => { localStorage.setItem("visto-theme", theme); }, [theme]);
  useEffect(() => { fetch("/api/v1/me").then(response => response.ok ? response.json() : null).then(setUser).catch(() => setUser(null)); }, []);
  return <MantineProvider defaultColorScheme={theme === "system" ? "auto" : theme}>{user === undefined ? <Group justify="center" mt="xl"><Loader /></Group> : user === null ? <Login onLogin={setUser} /> : <Dashboard user={user} theme={theme} setTheme={setTheme} />}</MantineProvider>;
}

function Login({ onLogin }: { onLogin: (user: User) => void }) {
  const [username, setUsername] = useState(""); const [password, setPassword] = useState(""); const [error, setError] = useState(""); const [loading, setLoading] = useState(false);
  async function submit(event: FormEvent) { event.preventDefault(); setLoading(true); setError(""); const response = await fetch("/api/v1/auth/login", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ username, password }) }); setLoading(false); if (!response.ok) { setError("Invalid username or password."); return; } onLogin(await response.json()); }
  return <Paper withBorder radius="md" p="xl" maw={420} mx="auto" mt="xl"><Title order={1}>Welcome to Visto</Title><Text c="dimmed" mt="xs">Sign in to track what you watch.</Text><form onSubmit={submit}><TextInput required label="Username" value={username} onChange={event => setUsername(event.currentTarget.value)} mt="lg" /><PasswordInput required label="Password" value={password} onChange={event => setPassword(event.currentTarget.value)} mt="md" />{error && <Alert color="red" mt="md">{error}</Alert>}<Button type="submit" loading={loading} fullWidth mt="lg">Sign in</Button></form></Paper>;
}

function Dashboard({ user, theme, setTheme }: { user: User; theme: Theme; setTheme: (theme: Theme) => void }) {
  const [tab, setTab] = useState<Tab>("watch"); const [view, setView] = useState("now");
  const nav = (value: Tab, label: string, Icon: typeof IconHome) => <Button variant={tab === value ? "light" : "subtle"} leftSection={<Icon size={18} />} onClick={() => setTab(value)}>{label}</Button>;
  return <AppShell header={{ height: 64 }} footer={{ height: 70 }} padding="md"><AppShell.Header><Group h="100%" px="md" justify="space-between"><Title order={2}>Visto</Title><Group><Text size="sm">Hi, {user.display_name}</Text><Select value={theme} onChange={value => setTheme((value || "system") as Theme)} data={[{ value: "system", label: "System" }, { value: "light", label: "Light" }, { value: "dark", label: "Dark" }]} w={110} /></Group></Group></AppShell.Header><AppShell.Main>{tab === "watch" && <><Tabs value={view} onChange={value => setView(value || "now")}><Tabs.List><Tabs.Tab value="now">Now</Tabs.Tab><Tabs.Tab value="calendar" leftSection={<IconCalendar size={16} />}>Calendar</Tabs.Tab></Tabs.List></Tabs><Empty title={view === "now" ? "Nothing to continue yet" : "No upcoming episodes"} /></>}{tab === "search" && <><Title order={1}>Search</Title><TextInput mt="md" label="Search TMDB" placeholder="Movies and TV shows" leftSection={<IconSearch size={16} />} /><Empty title="Search results will appear here" /></>}{tab === "feed" && <Empty title="No shared activity yet" />}{tab === "library" && <Empty title="Your library is empty" />}</AppShell.Main><AppShell.Footer><Group justify="space-around" h="100%">{nav("watch", "Watch", IconHome)}{nav("search", "Search", IconSearch)}{nav("feed", "Feed", IconCompass)}{nav("library", "Library", IconLibrary)}</Group></AppShell.Footer></AppShell>;
}
function Empty({ title }: { title: string }) { return <Paper withBorder p="xl" mt="md" radius="md" ta="center"><Text fw={700}>{title}</Text></Paper>; }
