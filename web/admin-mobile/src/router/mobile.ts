import type { RouteRecordRaw } from 'vue-router'

const mobileRoutes: RouteRecordRaw[] = [
  {
    path: '/m/login',
    name: 'MobileLogin',
    component: () => import('@/views/LoginPage.vue'),
    meta: { requiresAuth: false, title: '管理员登录' },
  },
  {
    path: '/m/setup',
    name: 'MobileSetup',
    component: () => import('@/views/SetupPage.vue'),
    meta: { requiresAuth: false, title: '系统初始化' },
  },
  {
    path: '/m',
    component: () => import('@/components/MobileLayout.vue'),
    meta: { requiresAuth: true },
    children: [
      // ===== 状态 Tab =====
      {
        path: '',
        name: 'MobileHome',
        component: () => import('@/views/status/DashboardPage.vue'),
        meta: { title: '状态', tab: 'status' },
      },
      {
        path: 'channel-health',
        name: 'MobileChannelHealth',
        component: () => import('@/views/status/ChannelHealthPage.vue'),
        meta: { title: '渠道健康', hideTabBar: true },
      },
      {
        path: 'alert-events',
        name: 'MobileAlertEvents',
        component: () => import('@/views/status/AlertEventsPage.vue'),
        meta: { title: '告警事件', hideTabBar: true },
      },

      // ===== 模型 Tab =====
      {
        path: 'models',
        name: 'MobileModels',
        component: () => import('@/views/models/ModelsTabPage.vue'),
        meta: { title: '模型管理', tab: 'models' },
      },
      {
        path: 'models/:id',
        name: 'MobileModelDetail',
        component: () => import('@/views/models/ModelDetailPage.vue'),
        meta: { title: '模型详情', hideTabBar: true },
      },
      {
        path: 'channels/:id',
        name: 'MobileChannelDetail',
        component: () => import('@/views/models/ChannelDetailPage.vue'),
        meta: { title: '渠道详情', hideTabBar: true },
      },

      // ===== 运营 Tab =====
      {
        path: 'ops',
        name: 'MobileOps',
        component: () => import('@/views/ops/OpsMenuPage.vue'),
        meta: { title: '运营', tab: 'ops' },
      },

      // --- 运营子页: Wave 1 ---
      {
        path: 'error-logs',
        name: 'MobileErrorLogs',
        component: () => import('@/views/ops/ErrorLogsPage.vue'),
        meta: { title: '错误日志', hideTabBar: true },
      },
      {
        path: 'billing',
        name: 'MobileBilling',
        component: () => import('@/views/ops/BillingRecordsPage.vue'),
        meta: { title: '账单记录', hideTabBar: true },
      },
      {
        path: 'usage-logs',
        name: 'MobileUsageLogs',
        component: () => import('@/views/ops/UsageLogsPage.vue'),
        meta: { title: '请求日志', hideTabBar: true },
      },
      {
        path: 'wallets',
        name: 'MobileWallets',
        component: () => import('@/views/ops/WalletListPage.vue'),
        meta: { title: '钱包管理', hideTabBar: true },
      },
      {
        path: 'plugins',
        name: 'MobilePlugins',
        component: () => import('@/views/ops/PluginListPage.vue'),
        meta: { title: '插件管理', hideTabBar: true },
      },

      // --- 运营子页: Wave 2 ---
      {
        path: 'tenants',
        name: 'MobileTenants',
        component: () => import('@/views/ops/TenantListPage.vue'),
        meta: { title: '租户管理', hideTabBar: true },
      },
      {
        path: 'tenants/:id',
        name: 'MobileTenantDetail',
        component: () => import('@/views/ops/TenantDetailPage.vue'),
        meta: { title: '租户详情', hideTabBar: true },
      },
      {
        path: 'users',
        name: 'MobileUsers',
        component: () => import('@/views/ops/UserListPage.vue'),
        meta: { title: '用户管理', hideTabBar: true },
      },
      {
        path: 'orders',
        name: 'MobileOrders',
        component: () => import('@/views/ops/OrderListPage.vue'),
        meta: { title: '订单管理', hideTabBar: true },
      },
      {
        path: 'orders/:id',
        name: 'MobileOrderDetail',
        component: () => import('@/views/ops/OrderDetailPage.vue'),
        meta: { title: '订单详情', hideTabBar: true },
      },
      {
        path: 'wallets/:tenantId',
        name: 'MobileWalletDetail',
        component: () => import('@/views/ops/WalletDetailPage.vue'),
        meta: { title: '钱包详情', hideTabBar: true },
      },
      {
        path: 'plans',
        name: 'MobilePlans',
        component: () => import('@/views/ops/PlanListPage.vue'),
        meta: { title: '套餐管理', hideTabBar: true },
      },
      {
        path: 'redemptions',
        name: 'MobileRedemptions',
        component: () => import('@/views/ops/RedemptionListPage.vue'),
        meta: { title: '兑换码管理', hideTabBar: true },
      },
      {
        path: 'promo-codes',
        name: 'MobilePromoCodes',
        component: () => import('@/views/ops/PromoCodeListPage.vue'),
        meta: { title: '优惠码管理', hideTabBar: true },
      },

      // --- 运营子页: Wave 3 ---
      {
        path: 'settings',
        name: 'MobileSettings',
        component: () => import('@/views/ops/SettingsPage.vue'),
        meta: { title: '系统设置', hideTabBar: true },
      },
      {
        path: 'tickets',
        name: 'MobileTickets',
        component: () => import('@/views/ops/TicketListPage.vue'),
        meta: { title: '工单管理', hideTabBar: true },
      },
      {
        path: 'tickets/:id',
        name: 'MobileTicketDetail',
        component: () => import('@/views/ops/TicketDetailPage.vue'),
        meta: { title: '工单详情', hideTabBar: true },
      },
      {
        path: 'audit',
        name: 'MobileAudit',
        component: () => import('@/views/ops/AuditPage.vue'),
        meta: { title: '安全审计', hideTabBar: true },
      },
      {
        path: 'notifications',
        name: 'MobileNotifications',
        component: () => import('@/views/ops/NotificationPage.vue'),
        meta: { title: '通知消息', hideTabBar: true },
      },

      // ===== 我的 Tab =====
      {
        path: 'profile',
        name: 'MobileProfile',
        component: () => import('@/views/profile/ProfilePage.vue'),
        meta: { title: '我的', tab: 'profile' },
      },
      {
        path: 'sessions',
        name: 'MobileSessions',
        component: () => import('@/views/profile/SessionsPage.vue'),
        meta: { title: '会话管理', hideTabBar: true },
      },
    ],
  },
]

export default mobileRoutes
