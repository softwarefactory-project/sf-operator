//go:build !ignore_autogenerated

// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

// Code generated by controller-gen. DO NOT EDIT.

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BaseSpec) DeepCopyInto(out *BaseSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BaseSpec.
func (in *BaseSpec) DeepCopy() *BaseSpec {
	if in == nil {
		return nil
	}
	out := new(BaseSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BaseStatus) DeepCopyInto(out *BaseStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BaseStatus.
func (in *BaseStatus) DeepCopy() *BaseStatus {
	if in == nil {
		return nil
	}
	out := new(BaseStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ConfigRepositoryLocationSpec) DeepCopyInto(out *ConfigRepositoryLocationSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ConfigRepositoryLocationSpec.
func (in *ConfigRepositoryLocationSpec) DeepCopy() *ConfigRepositoryLocationSpec {
	if in == nil {
		return nil
	}
	out := new(ConfigRepositoryLocationSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ElasticSearchConnection) DeepCopyInto(out *ElasticSearchConnection) {
	*out = *in
	if in.UseSSL != nil {
		in, out := &in.UseSSL, &out.UseSSL
		*out = new(bool)
		**out = **in
	}
	if in.VerifyCerts != nil {
		in, out := &in.VerifyCerts, &out.VerifyCerts
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ElasticSearchConnection.
func (in *ElasticSearchConnection) DeepCopy() *ElasticSearchConnection {
	if in == nil {
		return nil
	}
	out := new(ElasticSearchConnection)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FluentBitForwarderSpec) DeepCopyInto(out *FluentBitForwarderSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FluentBitForwarderSpec.
func (in *FluentBitForwarderSpec) DeepCopy() *FluentBitForwarderSpec {
	if in == nil {
		return nil
	}
	out := new(FluentBitForwarderSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GerritConnection) DeepCopyInto(out *GerritConnection) {
	*out = *in
	if in.VerifySSL != nil {
		in, out := &in.VerifySSL, &out.VerifySSL
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GerritConnection.
func (in *GerritConnection) DeepCopy() *GerritConnection {
	if in == nil {
		return nil
	}
	out := new(GerritConnection)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GitConnection) DeepCopyInto(out *GitConnection) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GitConnection.
func (in *GitConnection) DeepCopy() *GitConnection {
	if in == nil {
		return nil
	}
	out := new(GitConnection)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GitHubConnection) DeepCopyInto(out *GitHubConnection) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GitHubConnection.
func (in *GitHubConnection) DeepCopy() *GitHubConnection {
	if in == nil {
		return nil
	}
	out := new(GitHubConnection)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GitLabConnection) DeepCopyInto(out *GitLabConnection) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GitLabConnection.
func (in *GitLabConnection) DeepCopy() *GitLabConnection {
	if in == nil {
		return nil
	}
	out := new(GitLabConnection)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GitServerSpec) DeepCopyInto(out *GitServerSpec) {
	*out = *in
	in.Storage.DeepCopyInto(&out.Storage)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GitServerSpec.
func (in *GitServerSpec) DeepCopy() *GitServerSpec {
	if in == nil {
		return nil
	}
	out := new(GitServerSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LetsEncryptSpec) DeepCopyInto(out *LetsEncryptSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LetsEncryptSpec.
func (in *LetsEncryptSpec) DeepCopy() *LetsEncryptSpec {
	if in == nil {
		return nil
	}
	out := new(LetsEncryptSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LogServer) DeepCopyInto(out *LogServer) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LogServer.
func (in *LogServer) DeepCopy() *LogServer {
	if in == nil {
		return nil
	}
	out := new(LogServer)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LogServer) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LogServerList) DeepCopyInto(out *LogServerList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]LogServer, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LogServerList.
func (in *LogServerList) DeepCopy() *LogServerList {
	if in == nil {
		return nil
	}
	out := new(LogServerList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LogServerList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LogServerSpec) DeepCopyInto(out *LogServerSpec) {
	*out = *in
	if in.LetsEncrypt != nil {
		in, out := &in.LetsEncrypt, &out.LetsEncrypt
		*out = new(LetsEncryptSpec)
		**out = **in
	}
	in.Settings.DeepCopyInto(&out.Settings)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LogServerSpec.
func (in *LogServerSpec) DeepCopy() *LogServerSpec {
	if in == nil {
		return nil
	}
	out := new(LogServerSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LogServerSpecSettings) DeepCopyInto(out *LogServerSpecSettings) {
	*out = *in
	in.Storage.DeepCopyInto(&out.Storage)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LogServerSpecSettings.
func (in *LogServerSpecSettings) DeepCopy() *LogServerSpecSettings {
	if in == nil {
		return nil
	}
	out := new(LogServerSpecSettings)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LogServerStatus) DeepCopyInto(out *LogServerStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LogServerStatus.
func (in *LogServerStatus) DeepCopy() *LogServerStatus {
	if in == nil {
		return nil
	}
	out := new(LogServerStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MariaDBSpec) DeepCopyInto(out *MariaDBSpec) {
	*out = *in
	in.DBStorage.DeepCopyInto(&out.DBStorage)
	in.LogStorage.DeepCopyInto(&out.LogStorage)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MariaDBSpec.
func (in *MariaDBSpec) DeepCopy() *MariaDBSpec {
	if in == nil {
		return nil
	}
	out := new(MariaDBSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodepoolBuilderSpec) DeepCopyInto(out *NodepoolBuilderSpec) {
	*out = *in
	in.Storage.DeepCopyInto(&out.Storage)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodepoolBuilderSpec.
func (in *NodepoolBuilderSpec) DeepCopy() *NodepoolBuilderSpec {
	if in == nil {
		return nil
	}
	out := new(NodepoolBuilderSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodepoolLauncherSpec) DeepCopyInto(out *NodepoolLauncherSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodepoolLauncherSpec.
func (in *NodepoolLauncherSpec) DeepCopy() *NodepoolLauncherSpec {
	if in == nil {
		return nil
	}
	out := new(NodepoolLauncherSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodepoolSpec) DeepCopyInto(out *NodepoolSpec) {
	*out = *in
	out.Launcher = in.Launcher
	in.Builder.DeepCopyInto(&out.Builder)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodepoolSpec.
func (in *NodepoolSpec) DeepCopy() *NodepoolSpec {
	if in == nil {
		return nil
	}
	out := new(NodepoolSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PagureConnection) DeepCopyInto(out *PagureConnection) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PagureConnection.
func (in *PagureConnection) DeepCopy() *PagureConnection {
	if in == nil {
		return nil
	}
	out := new(PagureConnection)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Secret) DeepCopyInto(out *Secret) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Secret.
func (in *Secret) DeepCopy() *Secret {
	if in == nil {
		return nil
	}
	out := new(Secret)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SecretRef) DeepCopyInto(out *SecretRef) {
	*out = *in
	if in.SecretKeyRef != nil {
		in, out := &in.SecretKeyRef, &out.SecretKeyRef
		*out = new(Secret)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SecretRef.
func (in *SecretRef) DeepCopy() *SecretRef {
	if in == nil {
		return nil
	}
	out := new(SecretRef)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SoftwareFactory) DeepCopyInto(out *SoftwareFactory) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SoftwareFactory.
func (in *SoftwareFactory) DeepCopy() *SoftwareFactory {
	if in == nil {
		return nil
	}
	out := new(SoftwareFactory)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SoftwareFactory) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SoftwareFactoryList) DeepCopyInto(out *SoftwareFactoryList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]SoftwareFactory, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SoftwareFactoryList.
func (in *SoftwareFactoryList) DeepCopy() *SoftwareFactoryList {
	if in == nil {
		return nil
	}
	out := new(SoftwareFactoryList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SoftwareFactoryList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SoftwareFactorySpec) DeepCopyInto(out *SoftwareFactorySpec) {
	*out = *in
	if in.LetsEncrypt != nil {
		in, out := &in.LetsEncrypt, &out.LetsEncrypt
		*out = new(LetsEncryptSpec)
		**out = **in
	}
	if in.FluentBitLogForwarding != nil {
		in, out := &in.FluentBitLogForwarding, &out.FluentBitLogForwarding
		*out = new(FluentBitForwarderSpec)
		**out = **in
	}
	out.ConfigRepositoryLocation = in.ConfigRepositoryLocation
	in.Zuul.DeepCopyInto(&out.Zuul)
	in.Nodepool.DeepCopyInto(&out.Nodepool)
	in.Zookeeper.DeepCopyInto(&out.Zookeeper)
	in.Logserver.DeepCopyInto(&out.Logserver)
	in.MariaDB.DeepCopyInto(&out.MariaDB)
	in.GitServer.DeepCopyInto(&out.GitServer)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SoftwareFactorySpec.
func (in *SoftwareFactorySpec) DeepCopy() *SoftwareFactorySpec {
	if in == nil {
		return nil
	}
	out := new(SoftwareFactorySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SoftwareFactoryStatus) DeepCopyInto(out *SoftwareFactoryStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SoftwareFactoryStatus.
func (in *SoftwareFactoryStatus) DeepCopy() *SoftwareFactoryStatus {
	if in == nil {
		return nil
	}
	out := new(SoftwareFactoryStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StorageSpec) DeepCopyInto(out *StorageSpec) {
	*out = *in
	out.Size = in.Size.DeepCopy()
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StorageSpec.
func (in *StorageSpec) DeepCopy() *StorageSpec {
	if in == nil {
		return nil
	}
	out := new(StorageSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ZookeeperSpec) DeepCopyInto(out *ZookeeperSpec) {
	*out = *in
	in.Storage.DeepCopyInto(&out.Storage)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ZookeeperSpec.
func (in *ZookeeperSpec) DeepCopy() *ZookeeperSpec {
	if in == nil {
		return nil
	}
	out := new(ZookeeperSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ZuulExecutorSpec) DeepCopyInto(out *ZuulExecutorSpec) {
	*out = *in
	in.Storage.DeepCopyInto(&out.Storage)
	if in.Enabled != nil {
		in, out := &in.Enabled, &out.Enabled
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ZuulExecutorSpec.
func (in *ZuulExecutorSpec) DeepCopy() *ZuulExecutorSpec {
	if in == nil {
		return nil
	}
	out := new(ZuulExecutorSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ZuulMergerSpec) DeepCopyInto(out *ZuulMergerSpec) {
	*out = *in
	in.Storage.DeepCopyInto(&out.Storage)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ZuulMergerSpec.
func (in *ZuulMergerSpec) DeepCopy() *ZuulMergerSpec {
	if in == nil {
		return nil
	}
	out := new(ZuulMergerSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ZuulOIDCAuthenticatorSpec) DeepCopyInto(out *ZuulOIDCAuthenticatorSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ZuulOIDCAuthenticatorSpec.
func (in *ZuulOIDCAuthenticatorSpec) DeepCopy() *ZuulOIDCAuthenticatorSpec {
	if in == nil {
		return nil
	}
	out := new(ZuulOIDCAuthenticatorSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ZuulSchedulerSpec) DeepCopyInto(out *ZuulSchedulerSpec) {
	*out = *in
	in.Storage.DeepCopyInto(&out.Storage)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ZuulSchedulerSpec.
func (in *ZuulSchedulerSpec) DeepCopy() *ZuulSchedulerSpec {
	if in == nil {
		return nil
	}
	out := new(ZuulSchedulerSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ZuulSpec) DeepCopyInto(out *ZuulSpec) {
	*out = *in
	if in.OIDCAuthenticators != nil {
		in, out := &in.OIDCAuthenticators, &out.OIDCAuthenticators
		*out = make([]ZuulOIDCAuthenticatorSpec, len(*in))
		copy(*out, *in)
	}
	if in.GerritConns != nil {
		in, out := &in.GerritConns, &out.GerritConns
		*out = make([]GerritConnection, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.GitHubConns != nil {
		in, out := &in.GitHubConns, &out.GitHubConns
		*out = make([]GitHubConnection, len(*in))
		copy(*out, *in)
	}
	if in.GitLabConns != nil {
		in, out := &in.GitLabConns, &out.GitLabConns
		*out = make([]GitLabConnection, len(*in))
		copy(*out, *in)
	}
	if in.GitConns != nil {
		in, out := &in.GitConns, &out.GitConns
		*out = make([]GitConnection, len(*in))
		copy(*out, *in)
	}
	if in.PagureConns != nil {
		in, out := &in.PagureConns, &out.PagureConns
		*out = make([]PagureConnection, len(*in))
		copy(*out, *in)
	}
	if in.ElasticSearchConns != nil {
		in, out := &in.ElasticSearchConns, &out.ElasticSearchConns
		*out = make([]ElasticSearchConnection, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	in.Executor.DeepCopyInto(&out.Executor)
	in.Scheduler.DeepCopyInto(&out.Scheduler)
	out.Web = in.Web
	in.Merger.DeepCopyInto(&out.Merger)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ZuulSpec.
func (in *ZuulSpec) DeepCopy() *ZuulSpec {
	if in == nil {
		return nil
	}
	out := new(ZuulSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ZuulWebSpec) DeepCopyInto(out *ZuulWebSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ZuulWebSpec.
func (in *ZuulWebSpec) DeepCopy() *ZuulWebSpec {
	if in == nil {
		return nil
	}
	out := new(ZuulWebSpec)
	in.DeepCopyInto(out)
	return out
}
