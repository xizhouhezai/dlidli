import { createApp } from 'vue'
import ElementPlus from 'element-plus'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import 'virtual:uno.css'
import 'element-plus/dist/index.css'

import App from './App.vue'
import router from './router'
import { setupPermDirective } from './directives/perm'
import './styles/main.scss'

const app = createApp(App)
app.use(router)
app.use(ElementPlus, { locale: zhCn })
setupPermDirective(app)
app.mount('#app')
