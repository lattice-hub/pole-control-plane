/** 治理规则使用的服务订阅者视图；与配置中心 v1 的发布模型无关。 */
export interface VersionClient {
  version: number;
  subscribers: FileSubscriber[];
}

export interface FileSubscriber {
  id: string;
  host: string;
  release_name: string;
  version: string;
  client_type: string;
}
