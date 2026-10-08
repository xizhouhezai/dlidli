import { backgroundTaskManager } from '@kit.BackgroundTasksKit';
import { wantAgent, Want, WantAgent } from '@kit.AbilityKit';
import { common } from '@kit.AbilityKit';
import { BusinessError } from '@kit.BasicServicesKit';
import { Logger } from '../common/utils/Logger';

const TAG: string = 'BackgroundTransfer';
const REQUEST_CODE: number = 0xD1D1;

/**
 * 长时任务（持续任务）申请：`DATA_TRANSFER` 类型，HMY-51 必做项。
 *
 * 为什么必须有：HMY-50 实测「切后台/锁屏后分片仍推进」只说明系统上传任务与页面生命周期解耦，
 * **不代表进程安全**——同尺寸对照里出现过切后台瞬间进程被回收、后台阶段零进展。
 * 故不能只依赖该观察，必须显式申请长时任务（plan §6.1 第 4 条的坑 1）。
 *
 * 失败不阻断上传（申请被拒/低端设备不支持时上传照跑，只是不保证后台存活），
 * 记 warn 留痕。真机锁屏 5 分钟的保活边界仍须真机复验，见 plan §6.1。
 */
export class BackgroundTransfer {
  private started: boolean = false;

  /** 上传开始前调用；重复调用为幂等（第二次直接返回） */
  async start(context: common.UIAbilityContext): Promise<void> {
    if (this.started) {
      return;
    }
    try {
      const want: Want = {
        bundleName: context.abilityInfo.bundleName,
        abilityName: context.abilityInfo.name
      };
      const agent: WantAgent = await wantAgent.getWantAgent({
        wants: [want],
        actionType: wantAgent.OperationType.START_ABILITY,
        requestCode: REQUEST_CODE
      });
      await backgroundTaskManager.startBackgroundRunning(context,
        backgroundTaskManager.BackgroundMode.DATA_TRANSFER, agent);
      this.started = true;
      Logger.info(TAG, '已申请长时任务（DATA_TRANSFER），后台/锁屏期间继续上传');
    } catch (err) {
      const be: BusinessError = err as BusinessError;
      Logger.warn(TAG, `申请长时任务失败（不阻断上传）：code=${be.code} ${be.message}`);
    }
  }

  /** 上传收口或取消后调用，释放长时任务；未申请成功时为空操作 */
  async stop(context: common.UIAbilityContext): Promise<void> {
    if (!this.started) {
      return;
    }
    this.started = false;
    try {
      await backgroundTaskManager.stopBackgroundRunning(context);
      Logger.info(TAG, '已释放长时任务');
    } catch (err) {
      Logger.warn(TAG, `释放长时任务失败：${(err as BusinessError).message}`);
    }
  }
}
