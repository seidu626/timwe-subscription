import config from '../../auth_config.json';

const { domain, clientId, authorizationParams: { audience }, errorPath } = config as {
  domain: string;
  clientId: string;
  authorizationParams: {
    audience?: string;
  };
  errorPath: string;
};

const baseApiEndpoint = 'https://api.nouveauricheglobalgroup.com';
type BrowserProcess = {
  env?: Record<string, string | undefined>;
};

const browserProcess = (globalThis as {
  process?: BrowserProcess;
}).process;

const sentryEnv = browserProcess?.env ?? {};
const sentryDsn = sentryEnv['SENTRY_DSN'];
const sentryEnvironment = sentryEnv['SENTRY_ENVIRONMENT'] ?? 'production';
const sentryRelease = sentryEnv['SENTRY_RELEASE'];

export const environment = {
  production: true,
  appName: 'Nouveauriche Admin',
  envName: 'PROD',
  isDebugMode: false,
  clientId: 'ng_smgr==',
  ApiUrlPrefix: 'api/',
  baseApiEndpoint,
  subscriptionApiEndpoint: baseApiEndpoint,
  subscriptionExternalAdminApiEndpoint: baseApiEndpoint,
  notificationApiEndpoint: baseApiEndpoint,
  acquisitionApiEndpoint: baseApiEndpoint,
  cadenceEngineEndpoint: baseApiEndpoint,
  cadenceAdminToken: '',
  landingWebBaseUrl: 'https://landing.your-domain.com',
  identityEndpoint: 'https://identityserver.mtn.com.gh',
  adminTenantBootstrap: {
    platformAdminEmails: [],
    tenantWorkspaces: [],
  },
  auth: {
    domain,
    clientId,
    authorizationParams: {
      ...(audience ? { audience } : {}),
      redirect_uri: window.location.origin,
    },
    errorPath,
  },
  httpInterceptor: {
    allowedList: [{ uri: `${baseApiEndpoint}/*` }],
  },
  sentryDsn,
  sentryEnvironment,
  sentryRelease,
};
