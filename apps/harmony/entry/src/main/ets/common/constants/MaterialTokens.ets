import { uiMaterial } from '@kit.ArkUI';

/**
 * 沉浸光感（API 26 系统材质）token 工厂。
 *
 * 作用面硬约束：除 Toggle/Select/弹窗类外，系统材质只在 Navigation/NavDestination 标题栏
 * 与 barPosition: BarPosition.End 的底部 TabBar 生效，故本文件只服务这两处。
 * 层级取值依据官方《沉浸光感》规范：顶部悬浮 ULTRA_THIN、底部悬浮 THIN、弹出 THICK、半模态 ULTRA_THICK。
 */
export class DliMaterial {
  private static readonly supported: boolean = uiMaterial.isImmersiveMaterialSupported();

  static isSupported(): boolean {
    return DliMaterial.supported;
  }

  /** Navigation 标题栏：顶部悬浮，最薄一层 + 列表内容可从其下滚过 */
  static titleBar(): uiMaterial.ImmersiveMaterial | undefined {
    if (!DliMaterial.supported) {
      return undefined;
    }
    return new uiMaterial.ImmersiveMaterial({
      style: uiMaterial.ImmersiveStyle.ULTRA_THIN,
      interactive: true
    });
  }

  /** 底部悬浮 TabBar：THIN + 渐变蒙层（蒙层由 maskColor/maskHeight 提供） */
  static tabBar(): uiMaterial.ImmersiveMaterial | undefined {
    if (!DliMaterial.supported) {
      return undefined;
    }
    return new uiMaterial.ImmersiveMaterial({
      style: uiMaterial.ImmersiveStyle.THIN,
      interactive: true
    });
  }

  /** 关掉某个组件的系统材质；注意传 undefined 是「恢复默认」而非关闭 */
  static off(): uiMaterial.Material {
    return uiMaterial.Material.empty;
  }

  /** 应用级材质开关状态（module.json5 的 ohos.arkui.UIMaterial.state 生效结果） */
  static appState(): uiMaterial.MaterialState {
    return uiMaterial.getMaterialInfo().state;
  }

  /** 全局材质档位；低算力设备上 style/materialColor/colorInvert 可能不生效 */
  static globalLevel(): uiMaterial.MaterialLevel {
    return uiMaterial.getGlobalMaterialLevel();
  }
}
