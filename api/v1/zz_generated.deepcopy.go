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
func (in *ConfigLocationsSpec) DeepCopyInto(out *ConfigLocationsSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ConfigLocationsSpec.
func (in *ConfigLocationsSpec) DeepCopy() *ConfigLocationsSpec {
	if in == nil {
		return nil
	}
	out := new(ConfigLocationsSpec)
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
func (in *GerritSpec) DeepCopyInto(out *GerritSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GerritSpec.
func (in *GerritSpec) DeepCopy() *GerritSpec {
	if in == nil {
		return nil
	}
	out := new(GerritSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MosquittoSpec) DeepCopyInto(out *MosquittoSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MosquittoSpec.
func (in *MosquittoSpec) DeepCopy() *MosquittoSpec {
	if in == nil {
		return nil
	}
	out := new(MosquittoSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MurmurChannelSpec) DeepCopyInto(out *MurmurChannelSpec) {
	*out = *in
	out.Password = in.Password
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MurmurChannelSpec.
func (in *MurmurChannelSpec) DeepCopy() *MurmurChannelSpec {
	if in == nil {
		return nil
	}
	out := new(MurmurChannelSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MurmurSpec) DeepCopyInto(out *MurmurSpec) {
	*out = *in
	out.Password = in.Password
	if in.Channels != nil {
		in, out := &in.Channels, &out.Channels
		*out = make([]MurmurChannelSpec, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MurmurSpec.
func (in *MurmurSpec) DeepCopy() *MurmurSpec {
	if in == nil {
		return nil
	}
	out := new(MurmurSpec)
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
	out.ConfigLocations = in.ConfigLocations
	out.Gerrit = in.Gerrit
	in.Zuul.DeepCopyInto(&out.Zuul)
	in.Murmur.DeepCopyInto(&out.Murmur)
	out.Mosquitto = in.Mosquitto
	out.Telemetry = in.Telemetry
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
func (in *TelemetrySpec) DeepCopyInto(out *TelemetrySpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TelemetrySpec.
func (in *TelemetrySpec) DeepCopy() *TelemetrySpec {
	if in == nil {
		return nil
	}
	out := new(TelemetrySpec)
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
