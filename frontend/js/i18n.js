// ===== 国际化 (i18n) =====
// 翻译字典 + t/applyI18n/setLanguage/init
// 用法:
//   - HTML 文本: <span data-i18n="nav.overview">概览</span>
//   - HTML placeholder: <input data-i18n-placeholder="form.search" placeholder="搜索">
//   - HTML title: <span data-i18n-title="tip.help" title="帮助">?</span>
//   - JS: t('toast.saved') / t('toast.deleted', {n: 3})
(function(){
  'use strict';
  var DICT = {
    zh: {
      nav: {
        overview: '概览', logs: '运行日志', register: '注册', accounts: '邮箱池',
        about: '关于', settings: '设置', toggleTheme: '切换主题', checkUpdate: '检查更新',
        language: '语言：中文 (点击切换)',
        ip: 'IP 管理'
      },
      page: {
        overview: '概览', logs: '运行日志', register: '注册', accounts: '邮箱池',
        settings: '设置', ip: 'IP 管理'
      },
      ip: {
        proxy: '代理', direct: '直连', add: '+ 添加代理', refresh: '刷新',
        batchTest: '批量测试', batchDelete: '批量删除', searchPlaceholder: '搜索出口 IP / 地址 / 国家',
        allStatus: '全部状态', enabled: '启用', disabled: '停用',
        empty: '暂无代理，点击右上角添加。', addTitle: '添加代理', editTitle: '编辑代理',
        tabSingle: '单个添加', tabBatch: '批量添加', protocol: '协议',
        host: '主机', port: '端口', username: '用户名', password: '密码', save: '保存',
        test: '测试', edit: '编辑', delete: '删除',
        colAddress: '地址', colType: '类型',
        colLocation: '位置', colLatency: '延迟', colStatus: '状态', colActions: '操作',
        type: { datacenter: '机房', mobile: '移动', residential: '家宽' },
        resultTitle: '测试结果', resultScheme: '协议', resultIP: '出口 IP',
        resultCountry: '国家', resultRegion: '地区', resultCity: '城市',
        resultISP: '运营商', resultLatency: '延迟', resultError: '错误',
        batchPlaceholder: '每行一个代理，格式：scheme://[user:pass@]host:port',
        loading: '加载中...',
        available: '可用', failure: '失败',
        testing: '正在测试…', unavailable: '不可用', unknownError: '未知错误',
        testFailed: '测试失败', noProxies: '没有可测试的代理', testingN: '测试 {n} 个代理…',
        addFailed: '添加失败', saveFailed: '保存失败', added: '已添加', saved: '已保存',
        hostRequired: '主机不能为空', invalidBatch: '没有有效的代理行',
        batchDone: '完成：{added} 成功', batchDup: '，{n} 已存在', batchFail: '，{n} 失败',
        deleteTitle: '删除代理', deleteMsg: '确认从池中删除该代理？', deleted: '已删除',
        deleteFailed: '删除失败', selectFirst: '请选择要删除的代理',
        batchDeleteTitle: '批量删除', batchDeleteMsg: '确认删除选中的 {n} 个代理？',
        batchDelete: '批量删除', batchDeleteDone: '删除完成：{ok} 成功'
      },
      common: {
        loading: '加载中...', loadFailed: '加载失败', noData: '暂无数据',
        copy: '复制', cancel: '取消', delete: '删除',
        reset: '重置', clearAll: '清空全部', select: '选择', close: '关闭',
        test: '测试',
        prevPage: '上一页', nextPage: '下一页'
      },
      status: {
        idle: '空闲', running: '运行中', success: '成功', failed: '失败',
        unregistered: '未注册', pending: '待获取', fetching: '获取中',
        ready: '已就绪', suspended: '已封禁', untested: '未测试',
        available: '可用', unavailable: '不可用',
      },
      overview: {
        kiroAccounts: 'Kiro 账号数', successRate: '注册成功率', taskControl: '任务控制',
        liveStatus: '实时状态', progress: '进度', success: '成功', failed: '失败',
        elapsed: '已耗时', eta: '预计剩余', avg: '平均耗时', rate: '成功率',
        newTask: '新建任务', stop: '停止'
      },
      about: {
        currentVersion: '当前版本', latestVersion: '最新版本', releaseDate: '发布日期', author: '作者',
        newVersionFound: '发现新版本', joinGroup: '加入交流群', updateContent: '更新内容',
        updateNow: '查看版本', features: '版本特性', clickToUpdate: '前往 Releases 页面下载最新版本',
        sponsor: '赞助支持', sponsorDesc: '如果这个工具对你有帮助，欢迎请作者喝杯咖啡 ☕',
        wechatPay: '微信支付', alipay: '支付宝'
      },
      settings: {
        title: '设置', subtitle: '配置通知、运行策略和高级选项', save: '保存设置', saved: '设置已保存',
        general: '常规', notification: '通知', networkResilience: '邮箱与容错',
        emailProxyMode: '邮箱取件网络', emailProxy: '邮箱自定义代理', direct: '直连', followTask: '跟随任务代理', customProxy: '自定义代理', otpTimeout: '验证码等待时间', retryProfile: '重试策略', retryFast: '快速（不重试）', retryStandard: '标准（1 次）', retryStable: '稳定（2 次）', stopOnRisk: '风控错误时停止整批任务', stopOnRiskDesc: '避免继续消耗邮箱和代理额度',
        dataDir: '存储目录', dataDirDesc: '邮箱池、邮箱服务配置和代理池的存储位置；默认位于本机应用数据目录',
        dataDirPlaceholder: '默认存储路径',
        outputDir: '注册结果输出目录', outputDirDesc: '成功账号以明文 JSON 数组写入该目录下的 accounts.json',
        outputDirPlaceholder: '默认：文档/KiroX',
        proxy: '代理',
        proxyDesc: '所有注册请求走该代理；留空=直连。支持 http/https/socks5 完整 URL，也支持 host:port:user:pass、host:port、user:pass@host:port 等简写。',
        sound: '提示音', soundDesc: '任务结束时播放提示音', desktopNotification: '桌面通知', desktopNotificationDesc: '任务完成时发送 Windows 桌面通知', soundVolume: '提示音音量',
        appearance: '外观与更新', theme: '主题', themeSystem: '跟随系统', themeLight: '浅色', themeDark: '深色', language: '界面语言', autoUpdate: '启动时自动检查更新',
        maintenance: '维护', logRetention: '日志保留天数', moeExpiry: 'MoeMail 有效期（分钟）', persistentLogs: '持久化运行日志', persistentLogsDesc: '落盘前自动遮蔽验证码与令牌', autoProbe: '进入 IP 管理时探测新代理', openLogs: '打开日志目录', clearLogs: '清理日志', clearFingerprint: '清理指纹缓存', logsCleared: '日志已清理', fingerprintCleared: '指纹缓存已清理', clearLogsConfirm: '确定删除全部持久化日志吗？', clearFingerprintConfirm: '确定清理全部指纹缓存吗？',
        advanced: '高级配置', advancedDesc: '服务端点与底层网络参数', advancedInlineWarning: '这些参数直接影响注册协议，修改后请先用少量任务验证。', advancedWarningTitle: '打开高级配置？', advancedWarning: '修改服务端点或底层网络参数可能导致注册失败、账号风控或接口不可用。仅在明确知道参数用途时继续。', continueOpen: '继续打开', requestTimeout: '网络请求超时（秒）', fingerprintTTL: '指纹缓存时长（小时）', telemetry: '协议遥测',
        fingerprintAlgorithm: '指纹偏移曲线', fingerprintAlgorithmDesc: '控制各指纹域相对缓存身份的重采样强度', fpCompositeCurve: '综合偏移', fpAverageOffset: '平均偏移', fpControlPoint: '曲线控制点', fpBrowser: '浏览器', fpPlatform: '平台', fpPlugins: '插件', fpResources: 'CPU / 内存', fpGPU: 'GPU / WebGL', fpScreen: '屏幕', fpTimezone: '时区', fpCanvas: 'Canvas', fpMath: '数学运行时', fpSession: '会话', fpReuse: '复用缓存', fpRegenerate: '重新采样'
      },
      logs: { title: '运行日志', copyLog: '复制日志', empty: '暂无日志' },
      register: {
        newTask: '新建注册任务', count: '注册数量', concurrency: '并发数', delay: '延迟 (秒)',
        emailProvider: '邮箱提供商', outlook: '微软邮箱', cloudmail: 'Cloud-Mail',
        selectDomain: '选择域名', selectAllDomain: '全选域名',
        domainHint: '邮箱名将自动生成随机字符串',
        modeRandom: '随机', modeRoundRobin: '轮询',
        startBtn: '开始注册', stopBtn: '停止',
        icloud: 'iCloud',
      },
      accounts: {
        moemailTitle: 'MoeMail 临时邮箱', cloudmailTitle: 'Cloud-Mail 自部署邮箱', addConfig: '添加新配置',
        configName: '名称', optional: '(可选)', configNamePlaceholder: '自动生成',
        apiUrl: 'API URL', apiKey: 'API Key',
        testConnection: '测试连接', addConfigBtn: '添加配置',
        outlookTitle: '微软邮箱', count: '共', countUnit: '个',
        addAccount: '添加账号', clearRegistered: '清除已注册',
        thIndex: '#', thEmail: '邮箱地址', thStatus: '状态', thAddedAt: '添加时间', thActions: '操作',
        addModalTitle: '添加微软邮箱账号',
        importFile: '导入文件', selectTxt: '选择 TXT 文件', perLine: '每行一个账号',
        orManual: '或手动输入', manualInput: '手动输入',
        manualFormat: '格式：邮箱----密码----ClientID----RefreshToken----imap/graph，每行一个（不填最后一段默认 imap）',
        manualPlaceholder: 'user@outlook.com----password----clientid----refreshtoken',
        addToList: '添加到列表',
        inputRequired: '请先输入 Outlook 账号数据',
        addedSummary: '成功添加 {n} 个账号，当前共 {total} 个',
        importSummary: '成功导入 {n} 个账号，当前共 {total} 个',
        importFailed: '导入失败',
        icloudTitle: 'iCloud 邮箱', icloudAddTitle: '添加 iCloud 邮箱账号',
        icloudPerLine: '每行一个账号', icloudManualInput: '手动输入',
        icloudFormat: '格式：邮箱----iCloud 消息列表 URL，每行一个',
        icloudPlaceholder: 'user@icloud.com----https://apple55.top/messages/xxx/user@icloud.com',
        pagerInfo: '第 {cur} / {total} 页 (共 {n} 个)',
        emptyRow: '暂无邮箱账号',
        deleteTitle: '删除账号',
        deleteMsg: '确认删除账号 {email} ?',
        deleteConfirm: '确认删除',
        deletedOne: '账号已删除',
        clearAllTitle: '清空微软邮箱',
        clearAllMsg: '确认清空所有微软邮箱账号？此操作不可恢复！',
        clearAllConfirm: '确认清空',
        allCleared: '已清空所有账号',
        noRegistered: '没有已注册的账号',
        clearRegisteredTitle: '清除已注册',
        clearRegisteredMsg: '确认删除 {n} 个已注册（成功/失败）的账号？',
        mailnestTitle: 'MailNest 临时邮箱',
        projectCode: '项目代码',
      },
      modal: {
        updateTitle: '发现新版本', updateLater: '稍后', updateDownload: '前往下载'
      },
      toast: {
        copied: '已复制',
        copyFailed: '复制失败', operationFailed: '操作失败',
        proxySaved: '代理已保存', proxyCleared: '代理已清除',
        dataDirSet: '存储目录已设置', dataDirReset: '已重置为默认存储目录',
        outputDirSet: '输出目录已设置', outputDirReset: '已重置为默认输出目录',
        addFailed: '添加失败',
        deleteOk: '删除成功', deleteFailed: '删除失败',
        clearFailed: '清空失败',
        accountsDeleted: '已删除 {n} 个账号',
        taskStarted: '任务已启动', taskStartFailed: '启动失败',
        taskStopping: '正在停止任务...', taskStopFailed: '停止失败',
        upToDate: '当前已是最新版本', checkUpdateFailed: '检查更新失败',
        taskCompleteMsg: '{name} 任务完成！成功 {s} / 失败 {f} / 共 {t}',
        logCopied: '日志已复制', logEmpty: '暂无日志',
        languageChanged: '已切换语言'
      },
      moemail: {
        testing: '测试中...',
        testFailed: '连接失败',
        testFailedShort: '测试失败',
        cannotSaveUntilOk: '连接测试未通过，未保存配置：',
        requiredUrlKey: '请填写 API URL 和 API Key',
        requiredUrlKeyShort: '请填写 API URL 和 Key',
        nameExists: '配置名称已存在',
        emptyInline: '暂无配置，请在上方添加 MoeMail 配置',
        autoNamePrefix: '配置',
        summaryActive: '已配置 {n} 个，可用 {m} 个',
        summaryNone: '未配置',
        noDomainsHint: '暂无配置，请先在设置页添加',
        deleteConfigTitle: '删除配置',
        deleteConfigMsg: '确认删除配置 "{name}" 吗？',
        addedNamed: '已添加: {name}',
        connectedDomains: '连接成功，{n} 个域名',
        err403Short: 'API Key 权限不足',
        err401Short: 'API Key 无效',
        err404Short: 'API 地址错误',
        errTimeoutShort: '连接超时',
        testOkWithDomains: '连接成功，可用域名 {n} 个',
        testOkNoDomain: '连接成功，但未返回可用域名'
      },
      cloudmail: {
        domainHint: '域名将在保存或测试连接时自动从服务器拉取，无需手动填写。',
        summaryNone: '未配置',
        summaryActive: '已配置 {n} 个，可用 {m} 个',
        emptyInline: '暂无配置，请在上方添加 Cloud-Mail 配置',
        autoNamePrefix: '配置',
        adminEmail: '管理员邮箱',
        adminPassword: '管理员密码',
        requiredFields: '请填写 URL、管理员邮箱、密码',
        nameExists: '配置名称已存在',
        testing: '测试中...',
        testFailed: '连接失败',
        testFailedShort: '测试失败',
        cannotSaveUntilOk: '连接测试未通过，未保存配置：',
        connectedDomainsList: '连接成功，域名: {d}',
        connectedNoDomain: '连接成功，但服务器未返回域名（可能开启了 loginDomain 隐私开关）',
        testOkWithDomains: '连接成功，{n} 个域名',
        testOkNoDomain: '连接成功，但服务器未返回域名',
        addedNamed: '已添加: {name}',
        addedWithDomains: '已添加 {name}，{n} 个域名',
        deleteConfigTitle: '删除配置',
        deleteConfigMsg: '确认删除配置 "{name}" 吗？',
        noDomainsHint: '暂无配置，请先在邮箱池页添加',
        noActiveDomain: '暂无可用域名，请先测试 Cloud-Mail 配置'
      },
      mailnest: {
        requiredKeyProjectCodeShort: '请填写 Key 和 项目代码',
        balance: '测试成功，余额为 {n}',
        summaryNone: "未配置",
        summaryActive: "已配置",
      },
    },
    en: {
      nav: {
        overview: 'Overview', logs: 'Logs', register: 'Register', accounts: 'Emails',
        about: 'About', settings: 'Settings', toggleTheme: 'Toggle theme', checkUpdate: 'Check update',
        language: 'Language: English (click to switch)',
        ip: 'IPs'
      },
      page: {
        overview: 'Overview', logs: 'Logs', register: 'Register', accounts: 'Emails',
        settings: 'Settings', ip: 'IPs'
      },
      ip: {
        proxy: 'Proxy', direct: 'Direct', add: '+ Add proxy', refresh: 'Refresh',
        batchTest: 'Batch test', batchDelete: 'Batch delete', searchPlaceholder: 'Search exit IP / address / country',
        allStatus: 'All statuses', enabled: 'Enabled', disabled: 'Disabled',
        empty: 'No proxies yet. Click Add proxy above.', addTitle: 'Add proxy', editTitle: 'Edit proxy',
        tabSingle: 'Add single', tabBatch: 'Add batch', protocol: 'Protocol',
        host: 'Host', port: 'Port', username: 'Username', password: 'Password', save: 'Save',
        test: 'Test', edit: 'Edit', delete: 'Delete',
        colAddress: 'Address', colType: 'Type',
        colLocation: 'Location', colLatency: 'Latency', colStatus: 'Status', colActions: 'Actions',
        type: { datacenter: 'Datacenter', mobile: 'Mobile', residential: 'Residential' },
        resultTitle: 'Test Result', resultScheme: 'Protocol', resultIP: 'Exit IP',
        resultCountry: 'Country', resultRegion: 'Region', resultCity: 'City',
        resultISP: 'ISP', resultLatency: 'Latency', resultError: 'Error',
        batchPlaceholder: 'One proxy per line: scheme://[user:pass@]host:port',
        loading: 'Loading...',
        available: 'Available', failure: 'Failed',
        testing: 'Testing…', unavailable: 'Unavailable', unknownError: 'Unknown error',
        testFailed: 'Test failed', noProxies: 'No proxies to test', testingN: 'Testing {n} proxies…',
        addFailed: 'Add failed', saveFailed: 'Save failed', added: 'Added', saved: 'Saved',
        hostRequired: 'Host is required', invalidBatch: 'No valid proxy lines',
        batchDone: 'Done: {added} added', batchDup: ', {n} already exists', batchFail: ', {n} failed',
        deleteTitle: 'Delete proxy', deleteMsg: 'Remove this proxy from the pool?', deleted: 'Deleted',
        deleteFailed: 'Delete failed', selectFirst: 'Select proxies to delete',
        batchDeleteTitle: 'Batch delete', batchDeleteMsg: 'Delete {n} selected proxies?',
        batchDelete: 'Batch delete', batchDeleteDone: 'Deleted: {ok}'
      },
      common: {
        loading: 'Loading...', loadFailed: 'Failed to load', noData: 'No data',
        copy: 'Copy', cancel: 'Cancel', delete: 'Delete',
        reset: 'Reset', clearAll: 'Clear all', select: 'Select', close: 'Close',
        test: 'Test',
        prevPage: 'Prev', nextPage: 'Next'
      },
      status: {
        idle: 'Idle', running: 'Running', success: 'Success', failed: 'Failed',
        unregistered: 'Unregistered', pending: 'Pending', fetching: 'Fetching',
        ready: 'Ready', suspended: 'Suspended', untested: 'Untested',
        available: 'Available', unavailable: 'Unavailable',
      },
      overview: {
        kiroAccounts: 'Kiro accounts', successRate: 'Success rate', taskControl: 'Task control',
        liveStatus: 'Live status', progress: 'Progress', success: 'Success', failed: 'Failed',
        elapsed: 'Elapsed', eta: 'ETA', avg: 'Average', rate: 'Rate',
        newTask: 'New task', stop: 'Stop'
      },
      about: {
        currentVersion: 'Current', latestVersion: 'Latest', releaseDate: 'Released', author: 'Author',
        newVersionFound: 'New version available', joinGroup: 'Join group', updateContent: "What's new",
        updateNow: 'View release', features: 'Release notes', clickToUpdate: 'Open Releases to download the latest version',
        sponsor: 'Sponsor', sponsorDesc: 'If this tool helps you, consider buying the author a coffee ☕',
        wechatPay: 'WeChat Pay', alipay: 'Alipay'
      },
      settings: {
        title: 'Settings', subtitle: 'Configure notifications, runtime policies, and advanced options', save: 'Save settings', saved: 'Settings saved',
        general: 'General', notification: 'Notifications', networkResilience: 'Email and resilience',
        emailProxyMode: 'Mailbox network', emailProxy: 'Custom mailbox proxy', direct: 'Direct', followTask: 'Follow task proxy', customProxy: 'Custom proxy', otpTimeout: 'Verification-code timeout', retryProfile: 'Retry profile', retryFast: 'Fast (no retry)', retryStandard: 'Standard (1 retry)', retryStable: 'Stable (2 retries)', stopOnRisk: 'Stop the batch on risk errors', stopOnRiskDesc: 'Prevents further mailbox and proxy usage',
        dataDir: 'Data directory', dataDirDesc: 'Storage for mailbox pools, mail service settings, and the proxy pool; defaults to local app data',
        dataDirPlaceholder: 'Default path',
        outputDir: 'Output directory', outputDirDesc: 'Successful accounts are written to accounts.json in this directory',
        outputDirPlaceholder: 'Default: Documents/KiroX',
        proxy: 'Proxy',
        proxyDesc: 'All requests use this proxy; empty = direct. Accepts http/https/socks5 URLs or shortcuts like host:port:user:pass.',
        sound: 'Sound', soundDesc: 'Play a sound when a task ends', desktopNotification: 'Desktop notifications', desktopNotificationDesc: 'Send a Windows notification when a task completes', soundVolume: 'Sound volume',
        appearance: 'Appearance and updates', theme: 'Theme', themeSystem: 'System', themeLight: 'Light', themeDark: 'Dark', language: 'Language', autoUpdate: 'Check for updates at startup',
        maintenance: 'Maintenance', logRetention: 'Log retention days', moeExpiry: 'MoeMail lifetime (minutes)', persistentLogs: 'Persist runtime logs', persistentLogsDesc: 'Codes and tokens are redacted before writing', autoProbe: 'Probe new proxies when opening IP management', openLogs: 'Open log directory', clearLogs: 'Clear logs', clearFingerprint: 'Clear fingerprint cache', logsCleared: 'Logs cleared', fingerprintCleared: 'Fingerprint cache cleared', clearLogsConfirm: 'Delete all persisted logs?', clearFingerprintConfirm: 'Clear the entire fingerprint cache?',
        advanced: 'Advanced settings', advancedDesc: 'Service endpoints and low-level network options', advancedInlineWarning: 'These values directly affect the registration protocol. Validate changes with a small task first.', advancedWarningTitle: 'Open advanced settings?', advancedWarning: 'Changing service endpoints or low-level network options can break registration, trigger risk controls, or make APIs unavailable. Continue only if you understand the parameters.', continueOpen: 'Continue', requestTimeout: 'Request timeout (seconds)', fingerprintTTL: 'Fingerprint cache (hours)', telemetry: 'Protocol telemetry',
        fingerprintAlgorithm: 'Fingerprint offset curve', fingerprintAlgorithmDesc: 'Controls the resampling intensity of each fingerprint domain against the cached identity', fpCompositeCurve: 'Composite offset', fpAverageOffset: 'Average offset', fpControlPoint: 'Curve control point', fpBrowser: 'Browser', fpPlatform: 'Platform', fpPlugins: 'Plugins', fpResources: 'CPU / memory', fpGPU: 'GPU / WebGL', fpScreen: 'Screen', fpTimezone: 'Timezone', fpCanvas: 'Canvas', fpMath: 'Math runtime', fpSession: 'Session', fpReuse: 'Reuse cache', fpRegenerate: 'Resample'
      },
      logs: { title: 'Logs', copyLog: 'Copy logs', empty: 'No logs' },
      register: {
        newTask: 'New registration task', count: 'Count', concurrency: 'Concurrency', delay: 'Delay (s)',
        emailProvider: 'Email provider', outlook: 'Microsoft', cloudmail: 'Cloud-Mail',
        selectDomain: 'Select domain', selectAllDomain: 'Select all',
        domainHint: 'Email username is auto-generated as random string',
        modeRandom: 'Random', modeRoundRobin: 'Round-robin',
        startBtn: 'Start', stopBtn: 'Stop',
        icloud: 'iCloud',
      },
      accounts: {
        moemailTitle: 'MoeMail temp mail', cloudmailTitle: 'Cloud-Mail (self-hosted)', addConfig: 'Add config',
        configName: 'Name', optional: '(optional)', configNamePlaceholder: 'auto-generated',
        apiUrl: 'API URL', apiKey: 'API Key',
        testConnection: 'Test connection', addConfigBtn: 'Add config',
        outlookTitle: 'Microsoft', count: 'Total', countUnit: '',
        addAccount: 'Add account', clearRegistered: 'Clear registered',
        thIndex: '#', thEmail: 'Email', thStatus: 'Status', thAddedAt: 'Added', thActions: 'Actions',
        addModalTitle: 'Add Microsoft account',
        importFile: 'Import file', selectTxt: 'Select TXT file', perLine: 'One account per line',
        orManual: 'or paste manually', manualInput: 'Manual input',
        manualFormat: 'Format: email----password----ClientID----RefreshToken----imap/graph, one per line (default: imap)',
        manualPlaceholder: 'user@outlook.com----password----clientid----refreshtoken',
        addToList: 'Add to list',
        inputRequired: 'Please enter Outlook account data first',
        addedSummary: 'Added {n} accounts. Total now: {total}',
        icloudTitle: 'iCloud mail', icloudAddTitle: 'Add iCloud account',
        icloudPerLine: 'One account per line', icloudManualInput: 'Manual input',
        icloudFormat: 'Format: email----iCloud message list URL, one per line',
        icloudPlaceholder: 'user@icloud.com----https://apple55.top/messages/xxx/user@icloud.com',
        importSummary: 'Imported {n} accounts. Total now: {total}',
        importFailed: 'Import failed',
        pagerInfo: 'Page {cur} / {total} (Total {n})',
        emptyRow: 'No mail accounts',
        deleteTitle: 'Delete account',
        deleteMsg: 'Delete account {email}?',
        deleteConfirm: 'Confirm delete',
        deletedOne: 'Account deleted',
        clearAllTitle: 'Clear Microsoft accounts',
        clearAllMsg: 'Clear all Microsoft mail accounts? This cannot be undone.',
        clearAllConfirm: 'Confirm clear',
        allCleared: 'All accounts cleared',
        noRegistered: 'No registered accounts',
        clearRegisteredTitle: 'Clear registered',
        clearRegisteredMsg: 'Delete {n} registered (success/failed) accounts?',
        mailnestTitle: 'MailNest temp mail',
        projectCode: 'Project code',
      },
      modal: {
        updateTitle: 'New version available', updateLater: 'Later', updateDownload: 'Download'
      },
      toast: {
        copied: 'Copied',
        copyFailed: 'Copy failed', operationFailed: 'Operation failed',
        proxySaved: 'Proxy saved', proxyCleared: 'Proxy cleared',
        dataDirSet: 'Data directory set', dataDirReset: 'Data directory reset to default',
        outputDirSet: 'Output directory set', outputDirReset: 'Output directory reset to default',
        addFailed: 'Failed to add',
        deleteOk: 'Deleted', deleteFailed: 'Failed to delete',
        clearFailed: 'Failed to clear',
        accountsDeleted: 'Deleted {n} accounts',
        taskStarted: 'Task started', taskStartFailed: 'Failed to start',
        taskStopping: 'Stopping task...', taskStopFailed: 'Failed to stop',
        upToDate: 'You are on the latest version', checkUpdateFailed: 'Failed to check update',
        taskCompleteMsg: '{name} done! Success {s} / Failed {f} / Total {t}',
        logCopied: 'Logs copied', logEmpty: 'No logs to copy',
        languageChanged: 'Language switched'
      },
      moemail: {
        testing: 'Testing...',
        testFailed: 'Connection failed',
        testFailedShort: 'Test failed',
        cannotSaveUntilOk: 'Connection test failed; config not saved: ',
        requiredUrlKey: 'Please fill in API URL and API Key',
        requiredUrlKeyShort: 'Please fill in API URL and Key',
        nameExists: 'Config name already exists',
        emptyInline: 'No configs yet — add a MoeMail config above',
        autoNamePrefix: 'Config',
        summaryActive: '{n} configured · {m} available',
        summaryNone: 'Not configured',
        noDomainsHint: 'No configs yet; add one in Settings',
        deleteConfigTitle: 'Delete config',
        deleteConfigMsg: 'Delete config "{name}"?',
        addedNamed: 'Added: {name}',
        connectedDomains: 'Connected, {n} domains',
        err403Short: 'API Key permission denied',
        err401Short: 'API Key invalid',
        err404Short: 'API URL incorrect',
        errTimeoutShort: 'Timeout',
        testOkWithDomains: 'Connected. {n} domains available.',
        testOkNoDomain: 'Connected, but no domains returned'
      },
      cloudmail: {
        domainHint: 'Domains are auto-fetched from the server on save or test — no need to fill in manually.',
        summaryNone: 'Not configured',
        summaryActive: '{n} configured, {m} active',
        emptyInline: 'No configs yet. Add a Cloud-Mail config above.',
        autoNamePrefix: 'Config',
        adminEmail: 'Admin email',
        adminPassword: 'Admin password',
        requiredFields: 'URL, admin email and password are required',
        nameExists: 'Name already exists',
        testing: 'Testing...',
        testFailed: 'Connection failed',
        testFailedShort: 'Test failed',
        cannotSaveUntilOk: 'Connection test failed; config not saved: ',
        connectedDomainsList: 'Connected. Domains: {d}',
        connectedNoDomain: 'Connected, but server returned no domains (loginDomain privacy may be on).',
        testOkWithDomains: 'Connected, {n} domains',
        testOkNoDomain: 'Connected, but server returned no domains.',
        addedNamed: 'Added: {name}',
        addedWithDomains: 'Added {name}, {n} domains',
        deleteConfigTitle: 'Delete config',
        deleteConfigMsg: 'Delete config "{name}"?',
        noDomainsHint: 'No configs yet. Add one in the Emails page first.',
        noActiveDomain: 'No active domains. Please test your Cloud-Mail configs first.'
      },
      mailnest: {
        requiredKeyProjectCodeShort: 'Please enter the Key and Project Code',
        balance: 'Test successful, balance is {n}',
        summaryNone: "Not configured",
        summaryActive: "Configured",
      },
    },
    ja: {
      nav: {
        overview: '概要', logs: 'ログ', register: '登録', accounts: 'メール',
        about: '情報', settings: '設定', toggleTheme: 'テーマ切替', checkUpdate: '更新確認',
        language: '言語：日本語 (クリックで切替)',
        ip: 'IP管理'
      },
      page: {
        overview: '概要', logs: 'ログ', register: '登録', accounts: 'メール',
        settings: '設定', ip: 'IP管理'
      },
      ip: {
        proxy: 'プロキシ', direct: '直接接続', add: '+ プロキシを追加', refresh: '更新',
        batchTest: '一括テスト', batchDelete: '一括削除', searchPlaceholder: '出口 IP / アドレス / 国を検索',
        allStatus: 'すべての状態', enabled: '有効', disabled: '無効',
        empty: 'プロキシがありません。右上から追加してください。', addTitle: 'プロキシを追加', editTitle: 'プロキシを編集',
        tabSingle: '単数追加', tabBatch: '一括追加', protocol: 'プロトコル',
        host: 'ホスト', port: 'ポート', username: 'ユーザー名', password: 'パスワード', save: '保存',
        test: 'テスト', edit: '編集', delete: '削除',
        colAddress: 'アドレス', colType: '種別',
        colLocation: '場所', colLatency: '遅延', colStatus: '状態', colActions: '操作',
        type: { datacenter: 'DC', mobile: 'モバイル', residential: '住宅' },
        resultTitle: 'テスト結果', resultScheme: 'プロトコル', resultIP: '出口 IP',
        resultCountry: '国', resultRegion: '地域', resultCity: '都市',
        resultISP: 'ISP', resultLatency: '遅延', resultError: 'エラー',
        batchPlaceholder: '1行1プロキシ: scheme://[user:pass@]host:port',
        loading: '読み込み中...',
        available: '利用可能', failure: '失敗',
        testing: 'テスト中…', unavailable: '利用不可', unknownError: '不明なエラー',
        testFailed: 'テスト失敗', noProxies: 'テストするプロキシがありません', testingN: '{n} 件のプロキシをテスト…',
        addFailed: '追加失敗', saveFailed: '保存失敗', added: '追加しました', saved: '保存しました',
        hostRequired: 'ホストを入力してください', invalidBatch: '有効なプロキシ行がありません',
        batchDone: '完了：{added} 件追加', batchDup: '、{n} 件重複', batchFail: '、{n} 件失敗',
        deleteTitle: 'プロキシを削除', deleteMsg: 'このプロキシをプールから削除しますか？', deleted: '削除しました',
        deleteFailed: '削除失敗', selectFirst: '削除するプロキシを選択してください',
        batchDeleteTitle: '一括削除', batchDeleteMsg: '選択した {n} 件のプロキシを削除しますか？',
        batchDelete: '一括削除', batchDeleteDone: '削除完了：{ok} 件'
      },
      common: {
        loading: '読み込み中...', loadFailed: '読み込み失敗', noData: 'データなし',
        copy: 'コピー', cancel: 'キャンセル', delete: '削除',
        reset: 'リセット', clearAll: 'すべてクリア', select: '選択', close: '閉じる',
        test: 'テスト',
        prevPage: '前へ', nextPage: '次へ'
      },
      status: {
        idle: '待機', running: '実行中', success: '成功', failed: '失敗',
        unregistered: '未登録', pending: '待機中', fetching: '取得中',
        ready: '準備完了', suspended: '凍結', untested: '未テスト',
        available: '利用可能', unavailable: '利用不可',
      },
      overview: {
        kiroAccounts: 'Kiro アカウント数', successRate: '登録成功率', taskControl: 'タスク操作',
        liveStatus: 'リアルタイム状態', progress: '進行状況', success: '成功', failed: '失敗',
        elapsed: '経過時間', eta: '残り時間', avg: '平均', rate: '成功率',
        newTask: '新規タスク', stop: '停止'
      },
      about: {
        currentVersion: '現在のバージョン', latestVersion: '最新バージョン', releaseDate: 'リリース日', author: '作者',
        newVersionFound: '新しいバージョンがあります', joinGroup: 'グループに参加', updateContent: '更新内容',
        updateNow: 'リリースを見る', features: 'リリースノート', clickToUpdate: 'Releases ページから最新版をダウンロード',
        sponsor: 'スポンサー', sponsorDesc: '役に立ったら作者にコーヒーを ☕',
        wechatPay: 'WeChat Pay', alipay: 'Alipay'
      },
      settings: {
        title: '設定', subtitle: '通知、実行ポリシー、詳細設定を構成', save: '設定を保存', saved: '設定を保存しました',
        general: '一般', notification: '通知', networkResilience: 'メールと耐障害性',
        emailProxyMode: 'メール取得ネットワーク', emailProxy: 'メール用カスタムプロキシ', direct: '直接接続', followTask: 'タスクプロキシに従う', customProxy: 'カスタムプロキシ', otpTimeout: '認証コード待機時間', retryProfile: '再試行ポリシー', retryFast: '高速（再試行なし）', retryStandard: '標準（1 回）', retryStable: '安定（2 回）', stopOnRisk: 'リスクエラー時に一括停止', stopOnRiskDesc: 'メールとプロキシの追加消費を防ぎます',
        dataDir: 'データディレクトリ', dataDirDesc: 'メールボックス、メールサービス設定、プロキシプールの保存先。既定ではローカルアプリデータ内です',
        dataDirPlaceholder: 'デフォルトパス',
        outputDir: '出力ディレクトリ', outputDirDesc: '成功アカウントはこのディレクトリの accounts.json に書き出されます',
        outputDirPlaceholder: 'デフォルト：Documents/KiroX',
        proxy: 'プロキシ',
        proxyDesc: 'すべてのリクエストでこのプロキシを使用。空欄=直接接続。http/https/socks5 のURL、または host:port:user:pass などの省略形式に対応。',
        sound: '通知音', soundDesc: 'タスク終了時に通知音を鳴らす', desktopNotification: 'デスクトップ通知', desktopNotificationDesc: 'タスク完了時に Windows 通知を送信', soundVolume: '通知音量',
        appearance: '外観と更新', theme: 'テーマ', themeSystem: 'システム設定', themeLight: 'ライト', themeDark: 'ダーク', language: '表示言語', autoUpdate: '起動時に更新を確認',
        maintenance: 'メンテナンス', logRetention: 'ログ保持日数', moeExpiry: 'MoeMail 有効期間（分）', persistentLogs: '実行ログを保存', persistentLogsDesc: '保存前にコードとトークンを隠します', autoProbe: 'IP 管理を開くとき新規プロキシを検査', openLogs: 'ログフォルダを開く', clearLogs: 'ログを削除', clearFingerprint: '指紋キャッシュを削除', logsCleared: 'ログを削除しました', fingerprintCleared: '指紋キャッシュを削除しました', clearLogsConfirm: '保存済みログをすべて削除しますか？', clearFingerprintConfirm: '指紋キャッシュをすべて削除しますか？',
        advanced: '詳細設定', advancedDesc: 'サービスエンドポイントと低レベルネットワーク設定', advancedInlineWarning: '登録プロトコルへ直接影響します。変更後は少数タスクで確認してください。', advancedWarningTitle: '詳細設定を開きますか？', advancedWarning: 'サービスエンドポイントや低レベルネットワーク設定を変えると、登録失敗、リスク検出、API 利用不能の原因になります。用途を理解している場合のみ続行してください。', continueOpen: '続行', requestTimeout: '通信タイムアウト（秒）', fingerprintTTL: '指紋キャッシュ（時間）', telemetry: 'プロトコルテレメトリ',
        fingerprintAlgorithm: '指紋オフセット曲線', fingerprintAlgorithmDesc: 'キャッシュ済み ID に対する各指紋領域の再サンプリング強度を制御します', fpCompositeCurve: '総合オフセット', fpAverageOffset: '平均オフセット', fpControlPoint: '曲線制御点', fpBrowser: 'ブラウザ', fpPlatform: 'プラットフォーム', fpPlugins: 'プラグイン', fpResources: 'CPU / メモリ', fpGPU: 'GPU / WebGL', fpScreen: '画面', fpTimezone: 'タイムゾーン', fpCanvas: 'Canvas', fpMath: '数値演算', fpSession: 'セッション', fpReuse: 'キャッシュを再利用', fpRegenerate: '再サンプリング'
      },
      logs: { title: 'ログ', copyLog: 'ログをコピー', empty: 'ログなし' },
      register: {
        newTask: '新規登録タスク', count: '登録数', concurrency: '同時実行数', delay: '遅延 (秒)',
        emailProvider: 'メールプロバイダ', outlook: 'Microsoft', cloudmail: 'Cloud-Mail',
        selectDomain: 'ドメイン選択', selectAllDomain: 'すべて選択',
        domainHint: 'ユーザー名はランダム文字列で自動生成されます',
        modeRandom: 'ランダム', modeRoundRobin: 'ラウンドロビン',
        startBtn: '登録開始', stopBtn: '停止',
        icloud: 'iCloud',
      },
      accounts: {
        moemailTitle: 'MoeMail 使い捨てメール', cloudmailTitle: 'Cloud-Mail (自己ホスト型)', addConfig: '新規追加',
        configName: '名前', optional: '(任意)', configNamePlaceholder: '自動生成',
        apiUrl: 'API URL', apiKey: 'API Key',
        testConnection: '接続テスト', addConfigBtn: '設定を追加',
        outlookTitle: 'Microsoft メール', count: '合計', countUnit: '件',
        addAccount: 'アカウント追加', clearRegistered: '登録済みを削除',
        thIndex: '#', thEmail: 'メールアドレス', thStatus: '状態', thAddedAt: '追加日時', thActions: '操作',
        addModalTitle: 'Microsoft アカウントを追加',
        importFile: 'ファイル取り込み', selectTxt: 'TXTファイルを選択', perLine: '1行に1アカウント',
        orManual: 'または手入力', manualInput: '手入力',
        manualFormat: '形式：email----password----ClientID----RefreshToken----imap/graph (1行1件、省略時 imap)',
        manualPlaceholder: 'user@outlook.com----password----clientid----refreshtoken',
        addToList: 'リストに追加',
        inputRequired: 'Outlook アカウントを入力してください',
        addedSummary: '{n} 件のアカウントを追加 (合計 {total} 件)',
        importSummary: '{n} 件のアカウントを取り込み (合計 {total} 件)',
        icloudTitle: 'iCloud メール', icloudAddTitle: 'iCloud アカウントを追加',
        icloudPerLine: '1行に1アカウント', icloudManualInput: '手入力',
        icloudFormat: '形式：email----iCloud メッセージリスト URL (1行1件)',
        icloudPlaceholder: 'user@icloud.com----https://apple55.top/messages/xxx/user@icloud.com',
        importFailed: '取り込み失敗',
        pagerInfo: '{cur} / {total} ページ (合計 {n} 件)',
        emptyRow: 'メールアカウントなし',
        deleteTitle: 'アカウント削除',
        deleteMsg: 'アカウント {email} を削除しますか?',
        deleteConfirm: '削除確定',
        deletedOne: 'アカウントを削除しました',
        clearAllTitle: 'Microsoft メールをクリア',
        clearAllMsg: 'すべての Microsoft メールアカウントをクリアしますか? 元に戻せません。',
        clearAllConfirm: 'クリア確定',
        allCleared: 'すべてのアカウントをクリアしました',
        noRegistered: '登録済みアカウントはありません',
        clearRegisteredTitle: '登録済みを削除',
        clearRegisteredMsg: '{n} 件の登録済み (成功/失敗) アカウントを削除しますか?',
        mailnestTitle: 'MailNest 使い捨てメール',
        projectCode: 'プロジェクトコード',
      },
      modal: {
        updateTitle: '新しいバージョンがあります', updateLater: '後で', updateDownload: 'ダウンロード'
      },
      toast: {
        copied: 'コピーしました',
        copyFailed: 'コピー失敗', operationFailed: '操作に失敗しました',
        proxySaved: 'プロキシを保存', proxyCleared: 'プロキシをクリア',
        dataDirSet: 'データディレクトリを設定', dataDirReset: 'データディレクトリをデフォルトに戻しました',
        outputDirSet: '出力ディレクトリを設定', outputDirReset: '出力ディレクトリをデフォルトに戻しました',
        addFailed: '追加に失敗しました',
        deleteOk: '削除しました', deleteFailed: '削除に失敗しました',
        clearFailed: 'クリアに失敗しました',
        accountsDeleted: '{n} 件のアカウントを削除',
        taskStarted: 'タスクを起動しました', taskStartFailed: '起動失敗',
        taskStopping: 'タスクを停止中...', taskStopFailed: '停止失敗',
        upToDate: '最新バージョンです', checkUpdateFailed: '更新確認失敗',
        taskCompleteMsg: '{name} 完了! 成功 {s} / 失敗 {f} / 合計 {t}',
        logCopied: 'ログをコピー', logEmpty: 'ログがありません',
        languageChanged: '言語を切り替えました'
      },
      moemail: {
        testing: 'テスト中...',
        testFailed: '接続失敗',
        testFailedShort: 'テスト失敗',
        cannotSaveUntilOk: '接続テスト未通過、設定を保存していません: ',
        requiredUrlKey: 'API URL と API Key を入力してください',
        requiredUrlKeyShort: 'API URL と Key を入力してください',
        nameExists: '設定名がすでに存在します',
        emptyInline: '設定がありません。上で MoeMail 設定を追加してください',
        autoNamePrefix: '設定',
        summaryActive: '{n} 件設定済み · 利用可能 {m} 件',
        summaryNone: '未設定',
        noDomainsHint: '設定がありません、設定ページで追加してください',
        deleteConfigTitle: '設定を削除',
        deleteConfigMsg: '設定 "{name}" を削除しますか?',
        addedNamed: '追加しました: {name}',
        connectedDomains: '接続成功、{n} 個のドメイン',
        err403Short: 'API Key 権限不足',
        err401Short: 'API Key 無効',
        err404Short: 'API URL 誤り',
        errTimeoutShort: 'タイムアウト',
        testOkWithDomains: '接続成功、利用可能ドメイン {n} 件',
        testOkNoDomain: '接続成功、ただしドメインなし'
      },
      cloudmail: {
        domainHint: 'ドメインは保存時または接続テスト時にサーバーから自動取得されます。手動入力は不要です。',
        summaryNone: '未設定',
        summaryActive: '{n} 件設定、{m} 件利用可',
        emptyInline: '設定なし。上のフォームから Cloud-Mail 設定を追加してください。',
        autoNamePrefix: '設定',
        adminEmail: '管理者メール',
        adminPassword: '管理者パスワード',
        requiredFields: 'URL、管理者メール、パスワードを入力してください',
        nameExists: '名前が既に存在します',
        testing: 'テスト中...',
        testFailed: '接続失敗',
        testFailedShort: 'テスト失敗',
        cannotSaveUntilOk: '接続テスト未通過、設定を保存していません: ',
        connectedDomainsList: '接続成功、ドメイン: {d}',
        connectedNoDomain: '接続成功、ただしサーバーがドメインを返しませんでした（loginDomain プライバシーが有効の可能性）',
        testOkWithDomains: '接続成功、{n} ドメイン',
        testOkNoDomain: '接続成功、ただしサーバーがドメインを返しませんでした',
        addedNamed: '追加しました: {name}',
        addedWithDomains: '追加しました {name}、{n} ドメイン',
        deleteConfigTitle: '設定を削除',
        deleteConfigMsg: '設定 "{name}" を削除しますか?',
        noDomainsHint: '設定なし。先にメールページで追加してください。',
        noActiveDomain: '利用可能なドメインがありません。先に Cloud-Mail 設定をテストしてください。'
      },
      mailnest: {
        requiredKeyProjectCodeShort: '「Key」と「プロジェクトコード」をご入力ください',
        balance: 'テストは成功しました。残高は {n} です。',
        summaryNone: "未設定",
        summaryActive: "設定済み",
      },
    }
  };
  var STORAGE_KEY = 'kirox-language';
  var DEFAULT_LANG = 'zh';
  var currentLang = DEFAULT_LANG;
  function getByPath(obj, path) {
    var parts = path.split('.');
    var cur = obj;
    for (var i = 0; i < parts.length; i++) {
      if (cur == null) return undefined;
      cur = cur[parts[i]];
    }
    return cur;
  }
  function interpolate(s, vars) {
    if (!vars) return s;
    return s.replace(/\{(\w+)\}/g, function(_, k) {
      return vars[k] != null ? vars[k] : '{' + k + '}';
    });
  }
  function t(key, vars) {
    var v = getByPath(DICT[currentLang], key);
    if (v == null) v = getByPath(DICT[DEFAULT_LANG], key);
    if (v == null) return key;
    return interpolate(v, vars);
  }
  function applyI18n(root) {
    root = root || document;
    // textContent
    var nodes = root.querySelectorAll('[data-i18n]');
    for (var i = 0; i < nodes.length; i++) {
      var el = nodes[i];
      var key = el.getAttribute('data-i18n');
      var val = t(key);
      // 保留 textContent 模式：完全替换
      el.textContent = val;
    }
    // placeholder
    var phs = root.querySelectorAll('[data-i18n-placeholder]');
    for (var j = 0; j < phs.length; j++) {
      phs[j].setAttribute('placeholder', t(phs[j].getAttribute('data-i18n-placeholder')));
    }
    // title (tooltips)
    var titles = root.querySelectorAll('[data-i18n-title]');
    for (var k = 0; k < titles.length; k++) {
      titles[k].setAttribute('title', t(titles[k].getAttribute('data-i18n-title')));
    }
    var ariaLabels = root.querySelectorAll('[data-i18n-aria-label]');
    for (var l = 0; l < ariaLabels.length; l++) {
      ariaLabels[l].setAttribute('aria-label', t(ariaLabels[l].getAttribute('data-i18n-aria-label')));
    }
  }
  function setLanguage(lang, options) {
    if (!DICT[lang]) lang = DEFAULT_LANG;
    currentLang = lang;
    try { localStorage.setItem(STORAGE_KEY, lang); } catch (e) {}
    document.documentElement.setAttribute('lang', lang);
    applyI18n(document);
    // 通知监听者
    try {
      var evt = new CustomEvent('i18n:changed', { detail: { lang: lang } });
      window.dispatchEvent(evt);
    } catch (e) {}
    // 持久化到后端（异步，不阻塞 UI）
    if (!options || options.persist !== false) {
      try {
        if (window.go && window.go.main && window.go.main.App && window.go.main.App.SetLanguage) {
          window.go.main.App.SetLanguage(lang);
        }
      } catch (e) {}
    }
  }
  function getLanguage() { return currentLang; }
  async function init() {
    var lang = '';
    // 1. 后端持久化值
    try {
      if (window.go && window.go.main && window.go.main.App && window.go.main.App.GetLanguage) {
        lang = await window.go.main.App.GetLanguage();
      }
    } catch (e) {}
    // 2. localStorage 回落
    if (!lang) {
      try { lang = localStorage.getItem(STORAGE_KEY) || ''; } catch (e) {}
    }
    // 3. OS 探测
    if (!lang) {
      try {
        if (window.go && window.go.main && window.go.main.App && window.go.main.App.GetOSLanguage) {
          lang = await window.go.main.App.GetOSLanguage();
        }
      } catch (e) {}
    }
    // 4. 浏览器语言
    if (!lang && navigator.language) {
      var nv = navigator.language.toLowerCase();
      if (nv.indexOf('zh') === 0) lang = 'zh';
      else if (nv.indexOf('ja') === 0) lang = 'ja';
      else lang = 'en';
    }
    if (!DICT[lang]) lang = DEFAULT_LANG;
    setLanguage(lang, { persist: false });
  }
  // ===== 日志短语翻译表 =====
  // 后端 log.Printf 输出的是中文；前端在渲染前按当前语言做替换。
  // phrases 按长→短排序，避免短语提前抢匹配（如「邮箱已被注册」 vs 「已注册」）。
  var LOG_PHRASES = {
    en: {
      tags: [
        ['[指纹]', '[Fingerprint]'],
        ['[验活]', '[Verify]']
      ],
      phrases: [
        ['密码已设置但验活失败，邮箱已消耗，不再重试', 'password set but verification failed; email consumed, no retry'],
        ['从新邮件中获取到验证码', 'got verification code from new email'],
        ['新邮件中未找到验证码', 'no verification code in new emails'],
        ['刷新 Token + 查用量 + 查模型', 'refresh Token + check usage + check models'],
        ['完成注册工作流', 'complete signup workflow'],
        ['Profile 页面初始化', 'Profile page init'],
        ['Signup API 初始化', 'Signup API init'],
        ['获取 SSO Token', 'get SSO token'],
        ['Kiro OIDC 授权', 'Kiro OIDC authorize'],
        ['Portal 初始化', 'Portal init'],
        ['工作流初始化', 'workflow init'],
        ['注册 (SIGNUP)', 'signup (SIGNUP)'],
        ['SSO 工作流', 'SSO workflow'],
        ['OIDC 注册', 'OIDC signup'],
        ['使用 Outlook', 'using Outlook'],
        ['Profile 启动', 'Profile start'],
        ['验证码已发送', 'verification code sent'],
        ['获取到验证码', 'got verification code'],
        ['发送验证码', 'send verification code'],
        ['认证成功', 'auth succeeded'],
        ['认证失败', 'auth failed'],
        ['认证异常', 'auth error'],
        ['设备授权', 'device authorize'],
        ['创建身份', 'create identity'],
        ['设置密码', 'set password'],
        ['查用量', 'check usage'],
        ['查模型', 'check models'],
        ['已发送', 'sent'],
        ['获取到', 'got'],
        ['初始化', 'init'],
        ['认证', 'auth'],
        ['授权', 'authorize'],
        ['启动', 'start'],
        ['刷新', 'refresh'],
        ['发送', 'send'],
        ['获取', 'get'],
        ['/个', '/each'],
        ['警告：无法获取系统配置', 'warning: failed to fetch system config'],
        ['立即终止所有注册任务', 'aborting all registration tasks immediately'],
        ['代理请求失败，降级直连', 'proxy request failed, falling back to direct'],
        ['代理拨号失败，降级直连', 'proxy dial failed, falling back to direct'],
        ['已被封禁，已从输出文件移除', 'banned; removed from output file'],
        ['配置文件格式无效，已重置', 'invalid config format; reset'],
        ['尝试直接使用域名', 'trying domain directly'],
        ['获取初始邮件列表失败', 'failed to fetch initial email list'],
        ['获取初始邮件失败', 'failed to fetch initial emails'],
        ['获取邮件数量失败', 'failed to fetch email count'],
        ['获取系统配置失败', 'failed to fetch system config'],
        ['获取邮件失败', 'failed to fetch emails'],
        ['新邮件中未找到验证码', 'no verification code in new emails'],
        ['从新邮件中获取到验证码', 'got verification code from new email'],
        ['开始等待验证码', 'waiting for verification code'],
        ['等待验证码', 'waiting for verification code'],
        ['检测到熔断级错误', 'circuit-breaker level error detected'],
        ['已注册，标记并换号', 'already registered; marking and switching'],
        ['账号池已耗尽', 'account pool exhausted'],
        ['无可用账号，跳过', 'no available account; skipping'],
        ['自动使用已保存配置', 'auto-using saved config'],
        ['自动切换到', 'auto-switching to'],
        ['没有可用域名', 'no available domains'],
        ['邮箱已被注册', 'email already registered'],
        ['邮箱状态异常', 'email status abnormal'],
        ['邮箱创建完成', 'mailbox created'],
        ['账号已被封禁', 'account banned'],
        ['创建 MoeMail 邮箱', 'creating MoeMail mailbox'],
        ['创建 cloud-mail 邮箱', 'creating cloud-mail mailbox'],
        ['生成 MoeMail 邮箱失败', 'failed to create MoeMail mailbox'],
        ['生成 cloud-mail 邮箱失败', 'failed to create cloud-mail mailbox'],
        ['创建邮箱失败', 'failed to create mailbox'],
        ['创建用户失败', 'failed to create user'],
        ['提交邮箱', 'submit email'],
        ['重试获取登录凭证', 'retry: get login credentials'],
        ['重试授权Kiro访问', 'retry: authorize Kiro access'],
        ['重试获取访问令牌', 'retry: get access token'],
        ['启动并发任务', 'starting concurrent tasks'],
        ['启动串行任务', 'starting serial tasks'],
        ['任务完成', 'tasks complete'],
        ['失败明细', 'Failure breakdown'],
        ['总耗时', 'Total time'],
        ['平均耗时', 'Average time'],
        ['成功结果', 'Success result'],
        ['成功率', 'Success rate'],
        ['保存结果失败', 'failed to save results'],
        ['结果已保存', 'results saved'],
        ['MoeMail 域名池', 'MoeMail domain pool'],
        ['cloud-mail 域名池', 'cloud-mail domain pool'],
        ['已启用代理', 'proxy enabled'],
        ['暂无新邮件', 'no new emails'],
        ['发送前邮件数', 'email count before send'],
        ['初始邮件数', 'initial email count'],
        ['初始最大 emailId', 'initial max emailId'],
        ['基线设为 0', 'baseline set to 0'],
        ['注册成功', 'Registration succeeded'],
        ['注册完成', 'Registration complete'],
        ['注册失败', 'Registration failed'],
        ['准备重试', 'preparing to retry'],
        ['开始注册', 'starting registration'],
        ['验活成功', 'verification succeeded'],
        ['验活异常', 'verification exception'],
        ['验活失败', 'verification failed'],
        ['Token 刷新成功', 'Token refreshed'],
        ['Token 刷新失败', 'Token refresh failed'],
        ['端点查询失败', 'endpoint query failed'],
        ['端点查询异常', 'endpoint query exception'],
        ['连续', 'consecutively'],
        ['次 SELECT 失败，放弃等待', ' SELECT failures; giving up'],
        ['连接失败', 'connection failed'],
        ['请求失败', 'request failed'],
        ['等待退避', 'waiting backoff'],
        ['不可用', 'unavailable'],
        ['重试中', 'retrying'],
        ['跳过格式错误的行', 'skipping malformed line'],
        ['选择目录失败', 'select directory failed'],
        ['选择文件失败', 'select file failed'],
        ['账号封禁', 'account banned'],
        ['网络问题', 'network issue'],
        ['其他错误', 'other errors'],
        ['邮箱已注册', 'email already registered'],
        ['封新邮件', ' new emails'],
        ['当前', 'current'],
        ['并发数', 'concurrency'],
        ['验证码', 'verification code'],
        ['个域名', ' domain(s)'],
        ['个任务', ' task(s)'],
        ['个配置', ' config(s)'],
        ['个数据文件', ' data file(s)'],
        ['指纹', 'fingerprint'],
        ['内存', 'memory'],
        ['核心', 'cores'],
        ['分辨率', 'resolution'],
        ['已保存', 'saved'],
        ['已迁移', 'migrated'],
        ['账号', 'account'],
        ['邮箱', 'email'],
        ['配置', 'config'],
        ['域名', 'domain'],
        ['重试', 'retry'],
        ['失败', 'failed'],
        ['成功', 'success'],
        ['总计', 'Total'],
        ['熔断', 'circuit breaker'],
        ['共', 'total'],
        ['封', '']
      ],
      regexes: [
        [/第\s*(\d+)\s*次重试/g, 'attempt $1 retry']
      ]
    },
    ja: {
      tags: [
        ['[指纹]', '[フィンガープリント]'],
        ['[验活]', '[認証確認]']
      ],
      phrases: [
        ['密码已设置但验活失败，邮箱已消耗，不再重试', 'パスワード設定済みだがアクティベーション失敗、メール消費済みのため再試行しません'],
        ['从新邮件中获取到验证码', '新着メールから認証コード取得'],
        ['新邮件中未找到验证码', '新着メールに認証コードなし'],
        ['刷新 Token + 查用量 + 查模型', 'トークン更新 + 使用量確認 + モデル確認'],
        ['完成注册工作流', '登録ワークフロー完了'],
        ['Profile 页面初始化', 'Profile ページ初期化'],
        ['Signup API 初始化', 'Signup API 初期化'],
        ['获取 SSO Token', 'SSO トークン取得'],
        ['Kiro OIDC 授权', 'Kiro OIDC 認可'],
        ['Portal 初始化', 'Portal 初期化'],
        ['工作流初始化', 'ワークフロー初期化'],
        ['注册 (SIGNUP)', '登録 (SIGNUP)'],
        ['SSO 工作流', 'SSO ワークフロー'],
        ['OIDC 注册', 'OIDC 登録'],
        ['使用 Outlook', 'Outlook 使用'],
        ['Profile 启动', 'Profile 起動'],
        ['验证码已发送', '認証コード送信済み'],
        ['获取到验证码', '認証コード取得'],
        ['发送验证码', '認証コード送信'],
        ['认证成功', '認証成功'],
        ['认证失败', '認証失敗'],
        ['认证异常', '認証異常'],
        ['设备授权', 'デバイス認可'],
        ['创建身份', 'アイデンティティ作成'],
        ['设置密码', 'パスワード設定'],
        ['查用量', '使用量確認'],
        ['查模型', 'モデル確認'],
        ['已发送', '送信済み'],
        ['获取到', '取得'],
        ['初始化', '初期化'],
        ['认证', '認証'],
        ['授权', '認可'],
        ['启动', '起動'],
        ['刷新', '更新'],
        ['发送', '送信'],
        ['获取', '取得'],
        ['/个', '/件'],
        ['警告：无法获取系统配置', '警告: システム設定を取得できません'],
        ['立即终止所有注册任务', 'すべての登録タスクを即時終了'],
        ['代理请求失败，降级直连', 'プロキシ要求失敗、直接接続にフォールバック'],
        ['代理拨号失败，降级直连', 'プロキシダイヤル失敗、直接接続にフォールバック'],
        ['已被封禁，已从输出文件移除', '停止済み、出力ファイルから削除'],
        ['配置文件格式无效，已重置', '設定ファイル形式が無効、リセット'],
        ['尝试直接使用域名', 'ドメインを直接使用してみる'],
        ['获取初始邮件列表失败', '初期メール一覧の取得失敗'],
        ['获取初始邮件失败', '初期メール取得失敗'],
        ['获取邮件数量失败', 'メール件数取得失敗'],
        ['获取系统配置失败', 'システム設定取得失敗'],
        ['获取邮件失败', 'メール取得失敗'],
        ['新邮件中未找到验证码', '新着メールに認証コードなし'],
        ['从新邮件中获取到验证码', '新着メールから認証コード取得'],
        ['开始等待验证码', '認証コード待機開始'],
        ['等待验证码', '認証コード待機中'],
        ['检测到熔断级错误', 'サーキットブレーカー級エラーを検出'],
        ['已注册，标记并换号', '既に登録済み、マークして切替'],
        ['账号池已耗尽', 'アカウントプール枯渇'],
        ['无可用账号，跳过', '利用可能なアカウントなし、スキップ'],
        ['自动使用已保存配置', '保存済み設定を自動使用'],
        ['自动切换到', '自動切替先'],
        ['没有可用域名', '利用可能なドメインなし'],
        ['邮箱已被注册', 'メールは既に登録済み'],
        ['邮箱状态异常', 'メール状態異常'],
        ['邮箱创建完成', 'メール作成完了'],
        ['账号已被封禁', 'アカウント停止'],
        ['创建 MoeMail 邮箱', 'MoeMail メール作成中'],
        ['创建 cloud-mail 邮箱', 'cloud-mail メール作成中'],
        ['生成 MoeMail 邮箱失败', 'MoeMail メール作成失敗'],
        ['生成 cloud-mail 邮箱失败', 'cloud-mail メール作成失敗'],
        ['创建邮箱失败', 'メール作成失敗'],
        ['创建用户失败', 'ユーザー作成失敗'],
        ['提交邮箱', 'メール送信'],
        ['重试获取登录凭证', '再試行: ログイン認証情報を取得'],
        ['重试授权Kiro访问', '再試行: Kiro アクセス認可'],
        ['重试获取访问令牌', '再試行: アクセストークン取得'],
        ['启动并发任务', '並行タスク開始'],
        ['启动串行任务', '直列タスク開始'],
        ['任务完成', 'タスク完了'],
        ['失败明细', '失敗内訳'],
        ['总耗时', '総時間'],
        ['平均耗时', '平均時間'],
        ['成功结果', '成功結果'],
        ['成功率', '成功率'],
        ['保存结果失败', '結果保存失敗'],
        ['结果已保存', '結果保存済み'],
        ['MoeMail 域名池', 'MoeMail ドメインプール'],
        ['cloud-mail 域名池', 'cloud-mail ドメインプール'],
        ['已启用代理', 'プロキシ有効'],
        ['暂无新邮件', '新着メールなし'],
        ['发送前邮件数', '送信前メール件数'],
        ['初始邮件数', '初期メール件数'],
        ['初始最大 emailId', '初期最大 emailId'],
        ['基线设为 0', 'ベースライン 0 に設定'],
        ['注册成功', '登録成功'],
        ['注册完成', '登録完了'],
        ['注册失败', '登録失敗'],
        ['准备重试', '再試行準備中'],
        ['开始注册', '登録開始'],
        ['验活成功', 'アクティベーション成功'],
        ['验活异常', 'アクティベーション異常'],
        ['验活失败', 'アクティベーション失敗'],
        ['Token 刷新成功', 'トークン更新成功'],
        ['Token 刷新失败', 'トークン更新失敗'],
        ['端点查询失败', 'エンドポイント問い合わせ失敗'],
        ['端点查询异常', 'エンドポイント問い合わせ異常'],
        ['连续', '連続'],
        ['次 SELECT 失败，放弃等待', '回 SELECT 失敗、待機を断念'],
        ['连接失败', '接続失敗'],
        ['请求失败', '要求失敗'],
        ['等待退避', 'バックオフ待機'],
        ['不可用', '利用不可'],
        ['重试中', '再試行中'],
        ['跳过格式错误的行', '不正な形式の行をスキップ'],
        ['选择目录失败', 'ディレクトリ選択失敗'],
        ['选择文件失败', 'ファイル選択失敗'],
        ['账号封禁', 'アカウント停止'],
        ['网络问题', 'ネットワーク問題'],
        ['其他错误', 'その他エラー'],
        ['邮箱已注册', 'メール登録済み'],
        ['封新邮件', ' 件の新着メール'],
        ['当前', '現在'],
        ['并发数', '並行数'],
        ['验证码', '認証コード'],
        ['个域名', ' ドメイン'],
        ['个任务', ' タスク'],
        ['个配置', ' 設定'],
        ['个数据文件', ' データファイル'],
        ['指纹', 'フィンガープリント'],
        ['内存', 'メモリ'],
        ['核心', 'コア'],
        ['分辨率', '解像度'],
        ['已保存', '保存済み'],
        ['已迁移', '移行済み'],
        ['账号', 'アカウント'],
        ['邮箱', 'メール'],
        ['配置', '設定'],
        ['域名', 'ドメイン'],
        ['重试', '再試行'],
        ['失败', '失敗'],
        ['成功', '成功'],
        ['总计', '合計'],
        ['熔断', 'サーキットブレーカー'],
        ['共', '合計'],
        ['封', '']
      ],
      regexes: [
        [/第\s*(\d+)\s*次重试/g, '再試行 $1 回目']
      ]
    }
  };
  // translateLog: 把后端中文日志按当前语言替换为对应译文。
  // 中文模式或未知语言直接返回原文。
  function translateLog(text) {
    if (!text) return text;
    var lang = getLanguage();
    if (lang === 'zh' || !LOG_PHRASES[lang]) return text;
    var rules = LOG_PHRASES[lang];
    var out = text;
    var i;
    for (i = 0; i < rules.tags.length; i++) {
      out = out.split(rules.tags[i][0]).join(rules.tags[i][1]);
    }
    for (i = 0; i < rules.phrases.length; i++) {
      out = out.split(rules.phrases[i][0]).join(rules.phrases[i][1]);
    }
    for (i = 0; i < rules.regexes.length; i++) {
      out = out.replace(rules.regexes[i][0], rules.regexes[i][1]);
    }
    return out;
  }
  window.I18N = {
    t: t,
    applyI18n: applyI18n,
    setLanguage: setLanguage,
    getLanguage: getLanguage,
    init: init,
    DICT: DICT,
    translateLog: translateLog
  };
  // 顶层快捷函数
  window.t = t;
})();
