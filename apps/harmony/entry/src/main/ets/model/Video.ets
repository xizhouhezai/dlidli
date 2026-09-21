/**
 * 视频域端侧模型（字段名以 server 的 OpenAPI/实现为准，见 plan §4）。
 * 本期只落首页验证所需的分类模型；信息流与稿件详情模型随 M4-HMY-05 补齐。
 */
export interface Category {
  id: number;
  parent_id: number;
  name: string;
  sort: number;
  status: number;
}
