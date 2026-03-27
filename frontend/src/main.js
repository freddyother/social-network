import { createApp } from "vue"

import App from "./App.vue"
import router from "./router"
import "./assets/main.css"
import { applyThemePreference, loadStoredThemePreference } from "./theme"

applyThemePreference(loadStoredThemePreference())

createApp(App).use(router).mount("#app")
