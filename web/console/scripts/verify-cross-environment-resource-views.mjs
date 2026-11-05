import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const read = (relative) => fs.readFileSync(path.join(root, relative), 'utf8');
const assertIncludes = (source, expected, label) => {
    if (!source.includes(expected)) {
        throw new Error(`${label}: missing ${expected}`);
    }
};

const logicalServiceApi = read('src/services/logical_service.ts');
const configApi = read('src/services/unified_config.ts');
const serviceDetail = read('src/pages/Discovery/Services/Instance/ServiceDetail.tsx');
const configPage = read('src/pages/Configuration/Unified/index.tsx');

assertIncludes(logicalServiceApi, 'describeLogicalServiceEnvironments', 'logical service environment API seam');
assertIncludes(logicalServiceApi, 'bindServiceEnvironment', 'explicit service environment binding seam');
assertIncludes(configApi, 'environment: string', 'configuration is keyed by environment');
assertIncludes(configPage, "params.get('environment')", 'configuration URL must retain environment context');

assertIncludes(serviceDetail, 'EnvironmentResourceSwitcher', 'service detail environment switcher');
assertIncludes(serviceDetail, 'binding.service_name', 'service environment switching must carry runtime service name');
assertIncludes(serviceDetail, 'resolveServiceEnvironmentBinding', 'direct environment URL must resolve explicit binding');

console.log('cross-environment resource views verified');
