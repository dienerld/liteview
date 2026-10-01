import { createApp } from "vue";
import { createPinia } from "pinia";
import App from "./App.vue";
import "./style.css";

// Follow the OS color scheme (no manual toggle).
const media = window.matchMedia("(prefers-color-scheme: dark)");
const applyTheme = (dark: boolean) => document.documentElement.classList.toggle("dark", dark);
applyTheme(media.matches);
media.addEventListener("change", (e) => applyTheme(e.matches));

createApp(App).use(createPinia()).mount("#app");
