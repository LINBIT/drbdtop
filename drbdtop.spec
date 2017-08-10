%define name drbdtop
%define release 1
%define version 0.1
#%define buildroot %{_topdir}/BUILD/%{name}-%{version}

BuildRoot: %{buildroot}
Requires: drbd-utils >= 9.0.0
Summary: like top but for DRBD
License: GPLv2+
ExclusiveOS: linux
Name: %{name}
Version: %{version}
Release: %{release}
Source: %{name}-%{version}.tar.gz
Group: System Environment/Kernel

%description
like top but for DRBD

%prep
%setup -q

%build

%install
mkdir -p %{buildroot}/%{_sbindir}/
cp %{_builddir}/%{name}-%{version}/%{name} %{buildroot}/%{_sbindir}/

%files
%defattr(-,root,root)
	%{_sbindir}/%{name}

