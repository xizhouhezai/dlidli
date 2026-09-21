import { Logger } from './Logger';

const TAG = 'Toast';

/**
 * showToast 声明了 @throws（上下文未就绪等），包一层让调用点不必逐个 try-catch；
 * 提示失败只记日志，不影响主流程。
 */
export function showToast(ctx: UIContext, message: ResourceStr): void {
  try {
    ctx.getPromptAction().showToast({ message });
  } catch (err) {
    const cause: Error = err as Error;
    Logger.warn(TAG, `弹出提示失败：${cause.message}`);
  }
}
