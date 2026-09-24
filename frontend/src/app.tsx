import { useEffect, useMemo, useState } from "react";
import { AppBar, BottomNavigation, BottomNavigationAction, Box, Button, CssBaseline, Paper, Select, MenuItem, Tab, Tabs, TextField, ThemeProvider, Toolbar, Typography, createTheme } from "@mui/material";
import { CalendarMonth, Explore, HomeRounded, LibraryBooks, Search as SearchIcon } from "@mui/icons-material";

type Tab = "watch" | "search" | "feed" | "library";
type Theme = "system" | "light" | "dark";

const labels: Record<Tab, string> = { watch: "Watch", search: "Search", feed: "Feed", library: "Library" };

export function App() {
  const [tab, setTab] = useState<Tab>("watch");
  const [theme, setTheme] = useState<Theme>(() => (localStorage.getItem("visto-theme") as Theme) || "system");
  const [online, setOnline] = useState(navigator.onLine);
  useEffect(() => { document.documentElement.dataset.theme = theme; localStorage.setItem("visto-theme", theme); }, [theme]);
  useEffect(() => { const update = () => setOnline(navigator.onLine); window.addEventListener("online", update); window.addEventListener("offline", update); return () => { window.removeEventListener("online", update); window.removeEventListener("offline", update); }; }, []);
  const resolvedTheme = theme === "system" ? (matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light") : theme;
  const muiTheme = useMemo(() => createTheme({ palette: { mode: resolvedTheme, primary: { main: "#5368d9" } }, shape: { borderRadius: 12 } }), [resolvedTheme]);
  return <ThemeProvider theme={muiTheme}><CssBaseline /><Box className="app"><AppBar color="transparent" elevation={0} position="static"><Toolbar disableGutters><Typography variant="h5" sx={{ flexGrow: 1, letterSpacing: "-.06em", fontWeight: 800 }}>Visto</Typography><Select size="small" value={theme} onChange={event => setTheme(event.target.value as Theme)} inputProps={{ "aria-label": "Theme" }}><MenuItem value="system">System theme</MenuItem><MenuItem value="light">Light</MenuItem><MenuItem value="dark">Dark</MenuItem></Select></Toolbar></AppBar>{!online && <Paper role="status" className="offline">You are offline. Cached information may be out of date.</Paper>}<Box component="section">{tab === "watch" && <Watch />}{tab === "search" && <Search />}{tab === "feed" && <Feed />}{tab === "library" && <Library />}</Box><Paper component="nav" aria-label="Primary navigation" elevation={4} className="bottom-nav"><BottomNavigation value={tab} onChange={(_, next) => setTab(next)} showLabels><BottomNavigationAction value="watch" label="Watch" icon={<HomeRounded />} /><BottomNavigationAction value="search" label="Search" icon={<SearchIcon />} /><BottomNavigationAction value="feed" label="Feed" icon={<Explore />} /><BottomNavigationAction value="library" label="Library" icon={<LibraryBooks />} /></BottomNavigation></Paper></Box></ThemeProvider>;
}
function Watch() { const [view, setView] = useState("now"); return <><Tabs value={view} onChange={(_, value) => setView(value)} sx={{ mt: 2 }}><Tab value="now" label="Now" /><Tab value="calendar" icon={<CalendarMonth />} iconPosition="start" label="Calendar" /></Tabs><Typography variant="h4" sx={{ mt: 3, mb: 2 }}>{view === "now" ? "Keep watching" : "Upcoming episodes"}</Typography><Empty title={view === "now" ? "Nothing to continue yet" : "No upcoming episodes"} description={view === "now" ? "Find a show and add it to your library to start tracking." : "Shows you are watching will appear here."} /></> }
function Search() { return <><Typography variant="h4" sx={{ mt: 3, mb: 2 }}>Search</Typography><TextField fullWidth placeholder="Movies and TV shows" label="Search TMDB" /><Empty title="Search TMDB" description="Search results will appear here." /></> }
function Feed() { return <><Typography variant="h4" sx={{ mt: 3, mb: 2 }}>Family feed</Typography><Empty title="No shared activity yet" description="Activity appears here when a family member enables instance sharing." /></> }
function Library() { return <><Typography variant="h4" sx={{ mt: 3, mb: 2 }}>Library</Typography><Empty title="Your library is empty" description="Add movies and shows from Search. Your profile and settings live here too." /><Button variant="contained" sx={{ mt: 2 }}>Profile & settings</Button></> }
function Empty({ title, description }: { title: string; description: string }) { return <Paper variant="outlined" className="empty"><Typography variant="subtitle1" sx={{ fontWeight: 700 }}>{title}</Typography><Typography color="text.secondary">{description}</Typography></Paper>; }
