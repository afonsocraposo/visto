import { useEffect, useState } from "react";

type Tab = "watch" | "search" | "feed" | "library";
type Theme = "system" | "light" | "dark";

const labels: Record<Tab, string> = { watch: "Watch", search: "Search", feed: "Feed", library: "Library" };

export function App() {
  const [tab, setTab] = useState<Tab>("watch");
  const [theme, setTheme] = useState<Theme>(() => (localStorage.getItem("visto-theme") as Theme) || "system");
  const [online, setOnline] = useState(navigator.onLine);
  useEffect(() => { document.documentElement.dataset.theme = theme; localStorage.setItem("visto-theme", theme); }, [theme]);
  useEffect(() => { const update = () => setOnline(navigator.onLine); window.addEventListener("online", update); window.addEventListener("offline", update); return () => { window.removeEventListener("online", update); window.removeEventListener("offline", update); }; }, []);
  return <main className="app">
    <header><a className="wordmark" href="#watch" onClick={() => setTab("watch")}>Visto</a><select aria-label="Theme" value={theme} onChange={event => setTheme(event.target.value as Theme)}><option value="system">System theme</option><option value="light">Light</option><option value="dark">Dark</option></select></header>
    {!online && <p className="offline" role="status">You are offline. Cached information may be out of date.</p>}
    <section className="content">{tab === "watch" && <Watch />}{tab === "search" && <Search />}{tab === "feed" && <Feed />}{tab === "library" && <Library />}</section>
    <nav aria-label="Primary navigation">{(Object.keys(labels) as Tab[]).map(item => <button key={item} className={tab === item ? "active" : ""} onClick={() => setTab(item)}>{labels[item]}</button>)}</nav>
  </main>;
}
function Watch() { const [view, setView] = useState("now"); return <><div className="subnav"><button className={view === "now" ? "selected" : ""} onClick={() => setView("now")}>Now</button><button className={view === "calendar" ? "selected" : ""} onClick={() => setView("calendar")}>Calendar</button></div><h1>{view === "now" ? "Keep watching" : "Upcoming episodes"}</h1><Empty title={view === "now" ? "Nothing to continue yet" : "No upcoming episodes"} description={view === "now" ? "Find a show and add it to your library to start tracking." : "Shows you are watching will appear here."} /></> }
function Search() { return <><h1>Search</h1><input className="search" placeholder="Movies and TV shows" aria-label="Search movies and TV shows" /><Empty title="Search TMDB" description="Search results will appear here." /></> }
function Feed() { return <><h1>Family feed</h1><Empty title="No shared activity yet" description="Activity appears here when a family member enables instance sharing." /></> }
function Library() { return <><h1>Library</h1><Empty title="Your library is empty" description="Add movies and shows from Search. Your profile and settings live here too." /><button className="settings">Profile & settings</button></> }
function Empty({ title, description }: { title: string; description: string }) { return <div className="empty"><h2>{title}</h2><p>{description}</p></div>; }
