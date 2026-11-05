package apolloserver

// Apollo 配置模型和 Pole 的配置模型的映射

// Apollo 配置模型层级
// 应用（AppId）
// ├── 环境（Environment）
// │   ├── 集群（Cluster）
// │   │   ├── 命名空间（Namespace）
// │   │   │   ├── 配置项（Item1, Item2, ...）
// │   │   │   └── 配置项（Item3, Item4, ...）
// │   │   └── 命名空间（Namespace2）
// │   └── 集群（Cluster2）
// └── 环境（Environment2）

// Pole 配置身份为 ConfigKey(environment, group, name)。
// 每个身份包含完整定义草稿、不可变发布历史与一个正式版及多个灰度版。
// Apollo 读取选中的 RenderSnapshot，并在发布显式允许 Apollo 转换时渲染。

// Apollo 配置模型到 Pole 配置模型的映射关系
// 1. Cluster → DataCenter → default 依次定位 environment，仅在明确 ABSENT 时回退。
// 2. AppId 映射到 group。
// 3. Namespace 映射到 name，必要时仅在 ABSENT 后尝试不带 .properties 后缀的 name。
