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
// 全局兜底：render/watch/事件回调里未捕获的异常不应无声失败
app.config.errorHandler = (err, _instance, info) => {
  console.error('[admin] 未处理异常:', err, info)
}
app.use(router)
app.use(ElementPlus, { locale: zhCn })
setupPermDirective(app)
app.mount('#app')
