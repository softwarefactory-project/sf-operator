// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the keycloak configuration.

package controllers

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
)

const KC_PORT = 8080
const KC_PORT_NAME = "kc-port"
const KC_IMAGE = "quay.io/software-factory/keycloak:15.0.2"
const KC_DIR = "/opt/jboss/keycloak"

func (r *SFController) KCAdminJob(name string, command string) bool {
	var job batchv1.Job
	job_name := "kcadm-" + name
	found := r.GetM(job_name, &job)

	kcadm := fmt.Sprintf(
		"%s/bin/kcadm.sh %s --no-config --password $KC_ADMIN_PASS --realm master --server http://keycloak:%d/auth --user admin",
		KC_DIR, command, KC_PORT,
	)

	if !found {
		container := apiv1.Container{
			Name:  "kcadm-client",
			Image: KC_IMAGE,
			Command: []string{"sh", "-c", fmt.Sprintf(`
echo 'Running: %s'
set -xe
%s
`, kcadm, kcadm)},
			Env: []apiv1.EnvVar{
				create_secret_env("KC_ADMIN_PASS", "keycloak-admin-password", "keycloak-admin-password"),
			},
		}
		job := create_job(r.ns, job_name, container)

		r.log.V(1).Info("Creating job to ensure db", "name", name)
		r.CreateR(&job)
		return false
	} else if job.Status.Succeeded >= 1 {
		return true
	} else {
		r.log.V(1).Info("Waiting for kcadmin result")
		return false
	}
}

func (r *SFController) DeployKeycloak(enabled bool) bool {
	var dep appsv1.Deployment
	found := r.GetM("keycloak", &dep)
	if !found && enabled {
		r.log.V(1).Info("Deploying keycloak")
		r.EnsureSecret("keycloak-admin-password")
		_, db_ready := r.EnsureDB("keycloak")
		if db_ready {
			r.log.V(1).Info("Keycloak DB is ready")
			cm_data := make(map[string]string)
			cm_data["standalone.xml"] = KC_CONFIG
			r.EnsureConfigMap("keycloak", cm_data)
			dep = create_deployment(r.ns, "keycloak", "quay.io/software-factory/keycloak:15.0.2")
			dep.Spec.Template.Spec.Containers[0].Command = []string{
				// It seems like the entrypoint takes care of creating the initial admin user,
				// but it may be doing too much. Instead we should run the standalone.sh script,
				// and create the admin user manually using
				//   /opt/jboss/keycloak/bin/add-user-keycloak.sh
				"/opt/jboss/tools/docker-entrypoint.sh", "--server-config", "../../../../../../../etc/keycloak/standalone.xml"}
			dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
				create_container_port(KC_PORT, KC_PORT_NAME),
			}
			dep.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
				{
					Name:      "config-volume",
					MountPath: "/etc/keycloak",
				},
			}
			dep.Spec.Template.Spec.Volumes = []apiv1.Volume{
				create_volume_cm("config-volume", "keycloak-config-map"),
			}
			dep.Spec.Template.Spec.Containers[0].Env = []apiv1.EnvVar{
				create_env("KEYCLOAK_FRONTEND_URL", "https://"+r.cr.Spec.FQDN+"/auth"),
				create_env("PROXY_ADDRESS_FORWARDING", "true"),
				create_env("KEYCLOAK_HTTP_PORT", fmt.Sprintf("%v", KC_PORT)),
				create_env("DB_VENDOR", "mysql"),
				create_env("DB_ADDR", "mariadb"),
				create_env("DB_USER", "keycloak"),
				create_secret_env("DB_PASSWORD", "keycloak-db-password", "keycloak-db-password"),
				create_env("KEYCLOAK_USER", "admin"),
				create_secret_env("KEYCLOAK_PASSWORD", "keycloak-admin-password", "keycloak-admin-password"),
			}
			dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_http_probe("/health/ready", 9990)
			r.CreateR(&dep)
			srv := create_service(r.ns, "keycloak", "keycloak", KC_PORT, KC_PORT_NAME)
			r.CreateR(&srv)
		}
	} else if found {
		if !enabled {
			r.log.V(1).Info("Destroying keycloak")
			if err := r.Delete(r.ctx, &dep); err != nil {
				panic(err.Error())
			}
		}
	}
	if enabled {
		if dep.Status.ReadyReplicas > 0 {
			return r.KCAdminJob("create-default-realm", "create realms --set realm=SF --set enabled=true")
		} else {
			r.log.V(1).Info("Waiting for keycloak to be ready...")
			return false
		}
	} else {
		// The service is not enabled, so it is always ready.
		return true
	}
}

func (r *SFController) IngressKeycloak() []netv1.IngressRule {
	return []netv1.IngressRule{
		create_ingress_rule("keycloak."+r.cr.Spec.FQDN, "keycloak", KC_PORT),
		// TODO: add admin service
	}
}

const KC_CONFIG = `<?xml version='1.0' encoding='UTF-8'?>

<server xmlns="urn:jboss:domain:16.0">
    <extensions>
        <extension module="org.jboss.as.clustering.infinispan"/>
        <extension module="org.jboss.as.connector"/>
        <extension module="org.jboss.as.deployment-scanner"/>
        <extension module="org.jboss.as.ee"/>
        <extension module="org.jboss.as.ejb3"/>
        <extension module="org.jboss.as.jaxrs"/>
        <extension module="org.jboss.as.jmx"/>
        <extension module="org.jboss.as.jpa"/>
        <extension module="org.jboss.as.logging"/>
        <extension module="org.jboss.as.mail"/>
        <extension module="org.jboss.as.naming"/>
        <extension module="org.jboss.as.remoting"/>
        <extension module="org.jboss.as.security"/>
        <extension module="org.jboss.as.transactions"/>
        <extension module="org.jboss.as.weld"/>
        <extension module="org.keycloak.keycloak-server-subsystem"/>
        <extension module="org.wildfly.extension.bean-validation"/>
        <extension module="org.wildfly.extension.core-management"/>
        <extension module="org.wildfly.extension.elytron"/>
        <extension module="org.wildfly.extension.health"/>
        <extension module="org.wildfly.extension.io"/>
        <extension module="org.wildfly.extension.metrics"/>
        <extension module="org.wildfly.extension.request-controller"/>
        <extension module="org.wildfly.extension.security.manager"/>
        <extension module="org.wildfly.extension.undertow"/>
    </extensions>
    <management>
        <security-realms>
            <security-realm name="ManagementRealm">
                <authentication>
                    <local default-user="$local" skip-group-loading="true"/>
                    <properties path="mgmt-users.properties" relative-to="jboss.server.config.dir"/>
                </authentication>
                <authorization map-groups-to-roles="false">
                    <properties path="mgmt-groups.properties" relative-to="jboss.server.config.dir"/>
                </authorization>
            </security-realm>
            <security-realm name="ApplicationRealm">
                <server-identities>
                    <ssl>
                        <keystore path="application.keystore" relative-to="jboss.server.config.dir" keystore-password="password" alias="server" key-password="password" generate-self-signed-certificate-host="localhost"/>
                    </ssl>
                </server-identities>
                <authentication>
                    <local default-user="$local" allowed-users="*" skip-group-loading="true"/>
                    <properties path="application-users.properties" relative-to="jboss.server.config.dir"/>
                </authentication>
                <authorization>
                    <properties path="application-roles.properties" relative-to="jboss.server.config.dir"/>
                </authorization>
            </security-realm>
        </security-realms>
        <audit-log>
            <formatters>
                <json-formatter name="json-formatter"/>
            </formatters>
            <handlers>
                <file-handler name="file" formatter="json-formatter" path="audit-log.log" relative-to="jboss.server.data.dir"/>
            </handlers>
            <logger log-boot="true" log-read-only="false" enabled="false">
                <handlers>
                    <handler name="file"/>
                </handlers>
            </logger>
        </audit-log>
        <management-interfaces>
            <http-interface security-realm="ManagementRealm">
                <http-upgrade enabled="true"/>
                <socket-binding http="management-http"/>
            </http-interface>
        </management-interfaces>
        <access-control provider="simple">
            <role-mapping>
                <role name="SuperUser">
                    <include>
                        <user name="$local"/>
                    </include>
                </role>
            </role-mapping>
        </access-control>
    </management>
    <profile>
        <subsystem xmlns="urn:jboss:domain:logging:8.0">
            <console-handler name="CONSOLE">
                <formatter>
                    <named-formatter name="COLOR-PATTERN"/>
                </formatter>
            </console-handler>
            <logger category="com.arjuna">
                <level name="WARN"/>
            </logger>
            <logger category="io.jaegertracing.Configuration">
                <level name="WARN"/>
            </logger>
            <logger category="org.jboss.as.config">
                <level name="DEBUG"/>
            </logger>
            <logger category="sun.rmi">
                <level name="WARN"/>
            </logger>
            <logger category="org.keycloak">
                <level name="${env.KEYCLOAK_LOGLEVEL:INFO}"/>
            </logger>
            <root-logger>
                <level name="${env.ROOT_LOGLEVEL:INFO}"/>
                <handlers>
                    <handler name="CONSOLE"/>
                </handlers>
            </root-logger>
            <formatter name="PATTERN">
                <pattern-formatter pattern="%d{yyyy-MM-dd HH:mm:ss,SSS} %-5p [%c] (%t) %s%e%n"/>
            </formatter>
            <formatter name="COLOR-PATTERN">
                <pattern-formatter pattern="%K{level}%d{HH:mm:ss,SSS} %-5p [%c] (%t) %s%e%n"/>
            </formatter>
        </subsystem>
        <subsystem xmlns="urn:jboss:domain:bean-validation:1.0"/>
        <subsystem xmlns="urn:jboss:domain:core-management:1.0"/>
        <subsystem xmlns="urn:jboss:domain:datasources:6.0">
            <datasources>
                <datasource jndi-name="java:jboss/datasources/ExampleDS" pool-name="ExampleDS" enabled="true" use-java-context="true" statistics-enabled="${wildfly.datasources.statistics-enabled:${wildfly.statistics-enabled:false}}">
                    <connection-url>jdbc:h2:mem:test;DB_CLOSE_DELAY=-1;DB_CLOSE_ON_EXIT=FALSE</connection-url>
                    <driver>h2</driver>
                    <security>
                        <user-name>sa</user-name>
                        <password>sa</password>
                    </security>
                </datasource>
                <datasource jndi-name="java:jboss/datasources/KeycloakDS" pool-name="KeycloakDS" enabled="true" use-java-context="true" use-ccm="true">
                    <connection-url>jdbc:mysql://${env.DB_ADDR:mysql}:${env.DB_PORT:3306}/${env.DB_DATABASE:keycloak}${env.JDBC_PARAMS:}</connection-url>
                    <driver>mysql</driver>
                    <pool>
                        <flush-strategy>IdleConnections</flush-strategy>
                    </pool>
                    <security>
                        <user-name>${env.DB_USER:keycloak}</user-name>
                        <password>${env.DB_PASSWORD:password}</password>
                    </security>
                    <validation>
                        <check-valid-connection-sql>SELECT 1</check-valid-connection-sql>
                        <background-validation>true</background-validation>
                        <background-validation-millis>60000</background-validation-millis>
                    </validation>
                </datasource>
                <drivers>
                    <driver name="h2" module="com.h2database.h2">
                        <xa-datasource-class>org.h2.jdbcx.JdbcDataSource</xa-datasource-class>
                    </driver>
                    <driver name="mysql" module="com.mysql.jdbc">
                        <xa-datasource-class>com.mysql.cj.jdbc.MysqlXADataSource</xa-datasource-class>
                    </driver>
                </drivers>
            </datasources>
        </subsystem>
        <subsystem xmlns="urn:jboss:domain:deployment-scanner:2.0">
            <deployment-scanner path="deployments" relative-to="jboss.server.base.dir" scan-interval="5000" runtime-failure-causes-rollback="${jboss.deployment.scanner.rollback.on.failure:false}"/>
        </subsystem>
        <subsystem xmlns="urn:jboss:domain:ee:6.0">
            <spec-descriptor-property-replacement>false</spec-descriptor-property-replacement>
            <concurrent>
                <context-services>
                    <context-service name="default" jndi-name="java:jboss/ee/concurrency/context/default" use-transaction-setup-provider="true"/>
                </context-services>
                <managed-thread-factories>
                    <managed-thread-factory name="default" jndi-name="java:jboss/ee/concurrency/factory/default" context-service="default"/>
                </managed-thread-factories>
                <managed-executor-services>
                    <managed-executor-service name="default" jndi-name="java:jboss/ee/concurrency/executor/default" context-service="default" hung-task-termination-period="0" hung-task-threshold="60000" keepalive-time="5000"/>
                </managed-executor-services>
                <managed-scheduled-executor-services>
                    <managed-scheduled-executor-service name="default" jndi-name="java:jboss/ee/concurrency/scheduler/default" context-service="default" hung-task-termination-period="0" hung-task-threshold="60000" keepalive-time="3000"/>
                </managed-scheduled-executor-services>
            </concurrent>
            <default-bindings context-service="java:jboss/ee/concurrency/context/default" datasource="java:jboss/datasources/ExampleDS" managed-executor-service="java:jboss/ee/concurrency/executor/default" managed-scheduled-executor-service="java:jboss/ee/concurrency/scheduler/default" managed-thread-factory="java:jboss/ee/concurrency/factory/default"/>
        </subsystem>
        <subsystem xmlns="urn:jboss:domain:ejb3:9.0">
            <session-bean>
                <stateless>
                    <bean-instance-pool-ref pool-name="slsb-strict-max-pool"/>
                </stateless>
                <stateful default-access-timeout="5000" cache-ref="simple" passivation-disabled-cache-ref="simple"/>
                <singleton default-access-timeout="5000"/>
            </session-bean>
            <pools>
                <bean-instance-pools>
                    <strict-max-pool name="mdb-strict-max-pool" derive-size="from-cpu-count" instance-acquisition-timeout="5" instance-acquisition-timeout-unit="MINUTES"/>
                    <strict-max-pool name="slsb-strict-max-pool" derive-size="from-worker-pools" instance-acquisition-timeout="5" instance-acquisition-timeout-unit="MINUTES"/>
                </bean-instance-pools>
            </pools>
            <caches>
                <cache name="simple"/>
                <cache name="distributable" passivation-store-ref="infinispan" aliases="passivating clustered"/>
            </caches>
            <passivation-stores>
                <passivation-store name="infinispan" cache-container="ejb" max-size="10000"/>
            </passivation-stores>
            <async thread-pool-name="default"/>
            <timer-service thread-pool-name="default" default-data-store="default-file-store">
                <data-stores>
                    <file-data-store name="default-file-store" path="timer-service-data" relative-to="jboss.server.data.dir"/>
                </data-stores>
            </timer-service>
            <remote cluster="ejb" connectors="http-remoting-connector" thread-pool-name="default">
                <channel-creation-options>
                    <option name="MAX_OUTBOUND_MESSAGES" value="1234" type="remoting"/>
                </channel-creation-options>
            </remote>
            <thread-pools>
                <thread-pool name="default">
                    <max-threads count="10"/>
                    <keepalive-time time="60" unit="seconds"/>
                </thread-pool>
            </thread-pools>
            <default-security-domain value="other"/>
            <default-missing-method-permissions-deny-access value="true"/>
            <statistics enabled="${wildfly.ejb3.statistics-enabled:${wildfly.statistics-enabled:false}}"/>
            <log-system-exceptions value="true"/>
        </subsystem>
        <subsystem xmlns="urn:wildfly:elytron:13.0" final-providers="combined-providers" disallowed-providers="OracleUcrypto">
            <providers>
                <aggregate-providers name="combined-providers">
                    <providers name="elytron"/>
                    <providers name="openssl"/>
                </aggregate-providers>
                <provider-loader name="elytron" module="org.wildfly.security.elytron"/>
                <provider-loader name="openssl" module="org.wildfly.openssl"/>
            </providers>
            <audit-logging>
                <file-audit-log name="local-audit" path="audit.log" relative-to="jboss.server.log.dir" format="JSON"/>
            </audit-logging>
            <security-domains>
                <security-domain name="ApplicationDomain" default-realm="ApplicationRealm" permission-mapper="default-permission-mapper">
                    <realm name="ApplicationRealm" role-decoder="groups-to-roles"/>
                    <realm name="local"/>
                </security-domain>
                <security-domain name="ManagementDomain" default-realm="ManagementRealm" permission-mapper="default-permission-mapper">
                    <realm name="ManagementRealm" role-decoder="groups-to-roles"/>
                    <realm name="local" role-mapper="super-user-mapper"/>
                </security-domain>
            </security-domains>
            <security-realms>
                <identity-realm name="local" identity="$local"/>
                <properties-realm name="ApplicationRealm">
                    <users-properties path="application-users.properties" relative-to="jboss.server.config.dir" digest-realm-name="ApplicationRealm"/>
                    <groups-properties path="application-roles.properties" relative-to="jboss.server.config.dir"/>
                </properties-realm>
                <properties-realm name="ManagementRealm">
                    <users-properties path="mgmt-users.properties" relative-to="jboss.server.config.dir" digest-realm-name="ManagementRealm"/>
                    <groups-properties path="mgmt-groups.properties" relative-to="jboss.server.config.dir"/>
                </properties-realm>
            </security-realms>
            <mappers>
                <simple-permission-mapper name="default-permission-mapper" mapping-mode="first">
                    <permission-mapping>
                        <principal name="anonymous"/>
                        <permission-set name="default-permissions"/>
                    </permission-mapping>
                    <permission-mapping match-all="true">
                        <permission-set name="login-permission"/>
                        <permission-set name="default-permissions"/>
                    </permission-mapping>
                </simple-permission-mapper>
                <constant-realm-mapper name="local" realm-name="local"/>
                <simple-role-decoder name="groups-to-roles" attribute="groups"/>
                <constant-role-mapper name="super-user-mapper">
                    <role name="SuperUser"/>
                </constant-role-mapper>
            </mappers>
            <permission-sets>
                <permission-set name="login-permission">
                    <permission class-name="org.wildfly.security.auth.permission.LoginPermission"/>
                </permission-set>
                <permission-set name="default-permissions">
                    <permission class-name="org.wildfly.extension.batch.jberet.deployment.BatchPermission" module="org.wildfly.extension.batch.jberet" target-name="*"/>
                    <permission class-name="org.wildfly.transaction.client.RemoteTransactionPermission" module="org.wildfly.transaction.client"/>
                    <permission class-name="org.jboss.ejb.client.RemoteEJBPermission" module="org.jboss.ejb-client"/>
                    <permission class-name="org.jboss.ejb.client.RemoteEJBPermission" module="org.jboss.ejb-client"/>
                </permission-set>
            </permission-sets>
            <http>
                <http-authentication-factory name="management-http-authentication" security-domain="ManagementDomain" http-server-mechanism-factory="global">
                    <mechanism-configuration>
                        <mechanism mechanism-name="DIGEST">
                            <mechanism-realm realm-name="ManagementRealm"/>
                        </mechanism>
                    </mechanism-configuration>
                </http-authentication-factory>
                <provider-http-server-mechanism-factory name="global"/>
            </http>
            <sasl>
                <sasl-authentication-factory name="application-sasl-authentication" sasl-server-factory="configured" security-domain="ApplicationDomain">
                    <mechanism-configuration>
                        <mechanism mechanism-name="JBOSS-LOCAL-USER" realm-mapper="local"/>
                        <mechanism mechanism-name="DIGEST-MD5">
                            <mechanism-realm realm-name="ApplicationRealm"/>
                        </mechanism>
                    </mechanism-configuration>
                </sasl-authentication-factory>
                <sasl-authentication-factory name="management-sasl-authentication" sasl-server-factory="configured" security-domain="ManagementDomain">
                    <mechanism-configuration>
                        <mechanism mechanism-name="JBOSS-LOCAL-USER" realm-mapper="local"/>
                        <mechanism mechanism-name="DIGEST-MD5">
                            <mechanism-realm realm-name="ManagementRealm"/>
                        </mechanism>
                    </mechanism-configuration>
                </sasl-authentication-factory>
                <configurable-sasl-server-factory name="configured" sasl-server-factory="elytron">
                    <properties>
                        <property name="wildfly.sasl.local-user.default-user" value="$local"/>
                    </properties>
                </configurable-sasl-server-factory>
                <mechanism-provider-filtering-sasl-server-factory name="elytron" sasl-server-factory="global">
                    <filters>
                        <filter provider-name="WildFlyElytron"/>
                    </filters>
                </mechanism-provider-filtering-sasl-server-factory>
                <provider-sasl-server-factory name="global"/>
            </sasl>
            <tls>
                <key-stores>
                    <key-store name="applicationKS">
                        <credential-reference clear-text="password"/>
                        <implementation type="JKS"/>
                        <file path="application.keystore" relative-to="jboss.server.config.dir"/>
                    </key-store>
                </key-stores>
                <key-managers>
                    <key-manager name="applicationKM" key-store="applicationKS" generate-self-signed-certificate-host="localhost">
                        <credential-reference clear-text="password"/>
                    </key-manager>
                </key-managers>
                <server-ssl-contexts>
                    <server-ssl-context name="applicationSSC" key-manager="applicationKM"/>
                </server-ssl-contexts>
            </tls>
        </subsystem>
        <subsystem xmlns="urn:wildfly:health:1.0" security-enabled="false"/>
        <subsystem xmlns="urn:jboss:domain:infinispan:12.0">
            <cache-container name="ejb" default-cache="passivation" aliases="sfsb" modules="org.wildfly.clustering.ejb.infinispan">
                <local-cache name="passivation">
                    <locking isolation="REPEATABLE_READ"/>
                    <transaction mode="BATCH"/>
                    <file-store passivation="true" purge="false"/>
                </local-cache>
            </cache-container>
            <cache-container name="keycloak" modules="org.keycloak.keycloak-model-infinispan">
                <local-cache name="realms">
                    <heap-memory size="10000"/>
                </local-cache>
                <local-cache name="users">
                    <heap-memory size="10000"/>
                </local-cache>
                <local-cache name="sessions"/>
                <local-cache name="authenticationSessions"/>
                <local-cache name="offlineSessions"/>
                <local-cache name="clientSessions"/>
                <local-cache name="offlineClientSessions"/>
                <local-cache name="loginFailures"/>
                <local-cache name="work"/>
                <local-cache name="authorization">
                    <heap-memory size="10000"/>
                </local-cache>
                <local-cache name="keys">
                    <heap-memory size="1000"/>
                    <expiration max-idle="3600000"/>
                </local-cache>
                <local-cache name="actionTokens">
                    <heap-memory size="-1"/>
                    <expiration interval="300000" max-idle="-1"/>
                </local-cache>
            </cache-container>
            <cache-container name="server" default-cache="default" modules="org.wildfly.clustering.server">
                <local-cache name="default">
                    <transaction mode="BATCH"/>
                </local-cache>
            </cache-container>
            <cache-container name="web" default-cache="passivation" modules="org.wildfly.clustering.web.infinispan">
                <local-cache name="passivation">
                    <locking isolation="REPEATABLE_READ"/>
                    <transaction mode="BATCH"/>
                    <file-store passivation="true" purge="false"/>
                </local-cache>
                <local-cache name="sso">
                    <locking isolation="REPEATABLE_READ"/>
                    <transaction mode="BATCH"/>
                </local-cache>
                <local-cache name="routing"/>
            </cache-container>
            <cache-container name="hibernate" modules="org.infinispan.hibernate-cache">
                <local-cache name="entity">
                    <heap-memory size="10000"/>
                    <expiration max-idle="100000"/>
                </local-cache>
                <local-cache name="local-query">
                    <heap-memory size="10000"/>
                    <expiration max-idle="100000"/>
                </local-cache>
                <local-cache name="timestamps"/>
            </cache-container>
        </subsystem>
        <subsystem xmlns="urn:jboss:domain:io:3.0">
            <worker name="default"/>
            <buffer-pool name="default"/>
        </subsystem>
        <subsystem xmlns="urn:jboss:domain:jaxrs:2.0"/>
        <subsystem xmlns="urn:jboss:domain:jca:5.0">
            <archive-validation enabled="true" fail-on-error="true" fail-on-warn="false"/>
            <bean-validation enabled="true"/>
            <default-workmanager>
                <short-running-threads>
                    <core-threads count="50"/>
                    <queue-length count="50"/>
                    <max-threads count="50"/>
                    <keepalive-time time="10" unit="seconds"/>
                </short-running-threads>
                <long-running-threads>
                    <core-threads count="50"/>
                    <queue-length count="50"/>
                    <max-threads count="50"/>
                    <keepalive-time time="10" unit="seconds"/>
                </long-running-threads>
            </default-workmanager>
            <cached-connection-manager/>
        </subsystem>
        <subsystem xmlns="urn:jboss:domain:jmx:1.3">
            <expose-resolved-model/>
            <expose-expression-model/>
            <remoting-connector/>
        </subsystem>
        <subsystem xmlns="urn:jboss:domain:jpa:1.1">
            <jpa default-extended-persistence-inheritance="DEEP"/>
        </subsystem>
        <subsystem xmlns="urn:jboss:domain:keycloak-server:1.1">
            <web-context>auth</web-context>
            <providers>
                <provider>
                    classpath:${jboss.home.dir}/providers/*
                </provider>
            </providers>
            <master-realm-name>master</master-realm-name>
            <scheduled-task-interval>900</scheduled-task-interval>
            <theme>
                <staticMaxAge>2592000</staticMaxAge>
                <cacheThemes>true</cacheThemes>
                <cacheTemplates>true</cacheTemplates>
                <welcomeTheme>${env.KEYCLOAK_WELCOME_THEME:keycloak}</welcomeTheme>
                <default>${env.KEYCLOAK_DEFAULT_THEME:keycloak}</default>
                <dir>${jboss.home.dir}/themes</dir>
            </theme>
            <spi name="eventsStore">
                <provider name="jpa" enabled="true">
                    <properties>
                        <property name="exclude-events" value="[&quot;REFRESH_TOKEN&quot;]"/>
                    </properties>
                </provider>
            </spi>
            <spi name="userCache">
                <provider name="default" enabled="true"/>
            </spi>
            <spi name="userSessionPersister">
                <default-provider>jpa</default-provider>
            </spi>
            <spi name="timer">
                <default-provider>basic</default-provider>
            </spi>
            <spi name="connectionsHttpClient">
                <provider name="default" enabled="true"/>
            </spi>
            <spi name="connectionsJpa">
                <provider name="default" enabled="true">
                    <properties>
                        <property name="dataSource" value="java:jboss/datasources/KeycloakDS"/>
                        <property name="initializeEmpty" value="true"/>
                        <property name="migrationStrategy" value="update"/>
                        <property name="migrationExport" value="${jboss.home.dir}/keycloak-database-update.sql"/>
                    </properties>
                </provider>
            </spi>
            <spi name="realmCache">
                <provider name="default" enabled="true"/>
            </spi>
            <spi name="connectionsInfinispan">
                <default-provider>default</default-provider>
                <provider name="default" enabled="true">
                    <properties>
                        <property name="cacheContainer" value="java:jboss/infinispan/container/keycloak"/>
                    </properties>
                </provider>
            </spi>
            <spi name="jta-lookup">
                <default-provider>${keycloak.jta.lookup.provider:jboss}</default-provider>
                <provider name="jboss" enabled="true"/>
            </spi>
            <spi name="publicKeyStorage">
                <provider name="infinispan" enabled="true">
                    <properties>
                        <property name="minTimeBetweenRequests" value="10"/>
                    </properties>
                </provider>
            </spi>
            <spi name="x509cert-lookup">
                <default-provider>${keycloak.x509cert.lookup.provider:default}</default-provider>
                <provider name="default" enabled="true"/>
            </spi>
            <spi name="hostname">
                <default-provider>${keycloak.hostname.provider:default}</default-provider>
                <provider name="default" enabled="true">
                    <properties>
                        <property name="frontendUrl" value="${keycloak.frontendUrl:}"/>
                        <property name="forceBackendUrlToFrontendUrl" value="false"/>
                    </properties>
                </provider>
                <provider name="fixed" enabled="true">
                    <properties>
                        <property name="hostname" value="${keycloak.hostname.fixed.hostname:localhost}"/>
                        <property name="httpPort" value="${keycloak.hostname.fixed.httpPort:-1}"/>
                        <property name="httpsPort" value="${keycloak.hostname.fixed.httpsPort:-1}"/>
                        <property name="alwaysHttps" value="${keycloak.hostname.fixed.alwaysHttps:false}"/>
                    </properties>
                </provider>
            </spi>
        </subsystem>
        <subsystem xmlns="urn:jboss:domain:mail:4.0">
            <mail-session name="default" jndi-name="java:jboss/mail/Default">
                <smtp-server outbound-socket-binding-ref="mail-smtp"/>
            </mail-session>
        </subsystem>
        <subsystem xmlns="urn:wildfly:metrics:1.0" security-enabled="false" exposed-subsystems="*" prefix="${wildfly.metrics.prefix:wildfly}"/>
        <subsystem xmlns="urn:jboss:domain:naming:2.0">
            <remote-naming/>
        </subsystem>
        <subsystem xmlns="urn:jboss:domain:remoting:4.0">
            <http-connector name="http-remoting-connector" connector-ref="default" security-realm="ApplicationRealm"/>
        </subsystem>
        <subsystem xmlns="urn:jboss:domain:request-controller:1.0"/>
        <subsystem xmlns="urn:jboss:domain:security:2.0">
            <security-domains>
                <security-domain name="other" cache-type="default">
                    <authentication>
                        <login-module code="Remoting" flag="optional">
                            <module-option name="password-stacking" value="useFirstPass"/>
                        </login-module>
                        <login-module code="RealmDirect" flag="required">
                            <module-option name="password-stacking" value="useFirstPass"/>
                        </login-module>
                    </authentication>
                </security-domain>
                <security-domain name="jboss-web-policy" cache-type="default">
                    <authorization>
                        <policy-module code="Delegating" flag="required"/>
                    </authorization>
                </security-domain>
                <security-domain name="jaspitest" cache-type="default">
                    <authentication-jaspi>
                        <login-module-stack name="dummy">
                            <login-module code="Dummy" flag="optional"/>
                        </login-module-stack>
                        <auth-module code="Dummy"/>
                    </authentication-jaspi>
                </security-domain>
                <security-domain name="jboss-ejb-policy" cache-type="default">
                    <authorization>
                        <policy-module code="Delegating" flag="required"/>
                    </authorization>
                </security-domain>
            </security-domains>
        </subsystem>
        <subsystem xmlns="urn:jboss:domain:security-manager:1.0">
            <deployment-permissions>
                <maximum-set>
                    <permission class="java.security.AllPermission"/>
                </maximum-set>
            </deployment-permissions>
        </subsystem>
        <subsystem xmlns="urn:jboss:domain:transactions:6.0">
            <core-environment node-identifier="${jboss.tx.node.id:1}">
                <process-id>
                    <uuid/>
                </process-id>
            </core-environment>
            <recovery-environment socket-binding="txn-recovery-environment" status-socket-binding="txn-status-manager"/>
            <coordinator-environment statistics-enabled="${wildfly.transactions.statistics-enabled:${wildfly.statistics-enabled:false}}"/>
            <object-store path="tx-object-store" relative-to="jboss.server.data.dir"/>
        </subsystem>
        <subsystem xmlns="urn:jboss:domain:undertow:12.0" default-server="default-server" default-virtual-host="default-host" default-servlet-container="default" default-security-domain="other" statistics-enabled="${wildfly.undertow.statistics-enabled:${wildfly.statistics-enabled:false}}">
            <buffer-cache name="default"/>
            <server name="default-server">
                <http-listener name="default" socket-binding="http" redirect-socket="https" proxy-address-forwarding="${env.PROXY_ADDRESS_FORWARDING:false}" enable-http2="true"/>
                <https-listener name="https" socket-binding="https" proxy-address-forwarding="${env.PROXY_ADDRESS_FORWARDING:false}" security-realm="ApplicationRealm" enable-http2="true"/>
                <host name="default-host" alias="localhost">
                    <location name="/" handler="welcome-content"/>
                    <http-invoker security-realm="ApplicationRealm"/>
                </host>
            </server>
            <servlet-container name="default">
                <jsp-config/>
                <websockets/>
            </servlet-container>
            <handlers>
                <file name="welcome-content" path="${jboss.home.dir}/welcome-content"/>
            </handlers>
        </subsystem>
        <subsystem xmlns="urn:jboss:domain:weld:4.0"/>
    </profile>
    <interfaces>
        <interface name="management">
            <inet-address value="${jboss.bind.address.management:0.0.0.0}"/>
        </interface>
        <interface name="public">
            <inet-address value="${jboss.bind.address:0.0.0.0}"/>
        </interface>
    </interfaces>
    <socket-binding-group name="standard-sockets" default-interface="public" port-offset="${jboss.socket.binding.port-offset:0}">
        <socket-binding name="ajp" port="${jboss.ajp.port:8009}"/>
    <socket-binding name="http" port="${jboss.http.port:8080}"/>
        <socket-binding name="https" port="${jboss.https.port:8443}"/>
        <socket-binding name="management-http" interface="management" port="${jboss.management.http.port:9990}"/>
        <socket-binding name="management-https" interface="management" port="${jboss.management.https.port:9993}"/>
        <socket-binding name="txn-recovery-environment" port="4712"/>
        <socket-binding name="txn-status-manager" port="4713"/>
        <outbound-socket-binding name="mail-smtp">
            <remote-destination host="${jboss.mail.server.host:localhost}" port="${jboss.mail.server.port:25}"/>
        </outbound-socket-binding>
    </socket-binding-group>
</server>
`