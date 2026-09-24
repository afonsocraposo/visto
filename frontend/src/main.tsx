import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { App } from "./app/App";
import "@mantine/core/styles.css";
import "./styles.css";

const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: 60_000 } } });
if ("serviceWorker" in navigator) window.addEventListener("load", () => navigator.serviceWorker.register("/sw.js"));
createRoot(document.getElementById("root")!).render(<StrictMode><QueryClientProvider client={queryClient}><App /></QueryClientProvider></StrictMode>);
