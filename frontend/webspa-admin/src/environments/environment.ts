import config from '../../auth_config.json';

const { domain, clientId, authorizationParams: { audience }, errorPath } = config as {
  domain: string;
  clientId: string;
  authorizationParams: {
    audience?: string;
  },
  errorPath: string;
};

const baseApiEndpoint = 'http://localhost:5001';
const subscriptionApiEndpoint = 'http://localhost:8087';
const subscriptionExternalAdminApiEndpoint = 'http://localhost:8083';
const notificationApiEndpoint = 'http://localhost:8082';
const acquisitionApiEndpoint = 'http://localhost:8084';
const cadenceEngineEndpoint = 'http://localhost:8091';
const cadenceAdminToken = '790ea42af37b65d70d603e6a87b088a96ba37138562449d82fdebf4970920d33';
type BrowserProcess = {
  env?: Record<string, string | undefined>;
};

const browserProcess = (globalThis as {
  process?: BrowserProcess;
}).process;

const sentryEnv = browserProcess?.env ?? {};
const sentryDsn = sentryEnv['SENTRY_DSN'];
const sentryEnvironment = sentryEnv['SENTRY_ENVIRONMENT'] ?? 'development';
const sentryRelease = sentryEnv['SENTRY_RELEASE'];

export const environment = {
  production: false,
  appName: 'Nouveauriche Admin',
  envName: 'DEV',
  isDebugMode: true,
  clientId: 'h9dMo7AGADHmYBV5UAQio2yqv0TybPdJ',
  ApiUrlPrefix: 'api/',
  baseApiEndpoint,
  subscriptionApiEndpoint,
  subscriptionExternalAdminApiEndpoint,
  notificationApiEndpoint,
  acquisitionApiEndpoint,
  cadenceEngineEndpoint,
  cadenceAdminToken,
  landingWebBaseUrl: 'http://localhost:3000',
  identityEndpoint: 'https://identityserver.mtn.com.gh',
  adminTenantBootstrap: {
    platformAdminEmails: [
      'almauricin@gmail.com',
      'seidu.abdulai@hotmail.com',
    ],
    tenantWorkspaces: [
      {
        tenant_key: 'nrg',
        name: 'NRG',
      },
    ],
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
    allowedList: [
      // Use explicit { uri } entries so Auth0's interceptor reliably matches.
      { uri: `${baseApiEndpoint}/*` },
      { uri: `${subscriptionApiEndpoint}/*` },
      { uri: `${subscriptionExternalAdminApiEndpoint}/*` },
      { uri: `${notificationApiEndpoint}/*` },
      { uri: `${acquisitionApiEndpoint}/*` },
      // Cadence admin endpoints are under /v1/admin/cadence/*
      { uri: `${cadenceEngineEndpoint}/v1/admin/cadence/*` },
    ],
  },
  sentryDsn,
  sentryEnvironment,
  sentryRelease,
};


/*
 * For easier debugging in development mode, you can import the following file
 * to ignore zone related error stack frames such as `zone.run`, `zoneDelegate.invokeTask`.
 *
 * This import should be commented out in production mode because it will have a negative impact
 * on performance if an error is thrown.
 */
import 'zone.js/plugins/zone-error';  // Included with Angular CLI.
