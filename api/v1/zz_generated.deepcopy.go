//go:build !ignore_autogenerated
// +build !ignore_autogenerated

// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

// Code generated by controller-gen. DO NOT EDIT.

package v1

import (
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
func (in *ConfigLocationSpec) DeepCopyInto(out *ConfigLocationSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ConfigLocationSpec.
func (in *ConfigLocationSpec) DeepCopy() *ConfigLocationSpec {
	if in == nil {
		return nil
	}
	out := new(ConfigLocationSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GerritConnection) DeepCopyInto(out *GerritConnection) {
	*out = *in
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
func (in *LogServer) DeepCopyInto(out *LogServer) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
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
	out.SecretKeyRef = in.SecretKeyRef
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
	out.Status = in.Status
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
	out.ConfigLocation = in.ConfigLocation
	in.Zuul.DeepCopyInto(&out.Zuul)
	out.Nodepool = in.Nodepool
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
	if in.GerritConns != nil {
		in, out := &in.GerritConns, &out.GerritConns
		*out = make([]GerritConnection, len(*in))
		copy(*out, *in)
	}
	in.Executor.DeepCopyInto(&out.Executor)
	in.Scheduler.DeepCopyInto(&out.Scheduler)
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
