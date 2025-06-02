Name:           magellan
Version:        0.2.1
Release:        0%{?dist}
Summary:        Redfish-based, board management controller (BMC) discovery tool

License:        MIT License
URL:            https://github.com/OpenCHAMI/magellan
Source0:        %{name}-%{version}.tar.bz2

BuildRoot:      %{_tmppath}/%{name}-%{version}

BuildRequires:  golang-bin

%define _debugsource_template %{nil}

%description
The magellan CLI tool is a Redfish-based, board management controller (BMC)
discovery tool designed to scan networks and is written in Go. The tool
collects information from BMC nodes using the provided Redfish RESTful API
with gofish and loads the queried data into an SMD instance. The tool strives
to be more flexible by implementing multiple methods of discovery to work for
a wider range of systems (WIP) and is capable of being used independently of
other tools or services.

%prep

%setup -q

%build
go mod tidy 
go build

%install
%{__rm} -rf %{buildroot}
%{__install} -D -p -m 755 magellan %{buildroot}%{_bindir}/magellan

%post

%postun

%files
%license LICENSE
%doc README.md
%doc CHANGELOG.md
%{_bindir}/magellan
