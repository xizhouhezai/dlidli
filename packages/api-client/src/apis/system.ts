import type { HealthData } from '@dlidli/shared'
import type { HttpClient } from '../http'

/** 系统级接口（健康检查/连通性） */
export function createSystemApi(http: HttpClient) {
  return {
    health: () => http.get<HealthData>('/health'),
    ping: () => http.get<{ pong: number }>('/api/v1/ping'),
  }
}
