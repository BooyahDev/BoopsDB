Name: boops
Version: 0.1
Release: 1%{?dist}
Summary: Boops Client Application

License: MIT
URL: https://github.com/BooyahDev/BoopsDB
Source0: %{name}-%{version}.tar.gz

BuildArch: noarch

%description
Boops Client is a system monitoring and management application.

%prep
%setup -q

%build
go build -o boops main.go

%install
rm -rf %{buildroot}
mkdir -p %{buildroot}/usr/local/bin
cp boops %{buildroot}/usr/local/bin/

# Create config directory with proper permissions
mkdir -p /etc/boops
touch /etc/boops/config.json
chmod 644 /etc/boops/config.json

%post
# Copy systemd service file to /etc/systemd/system/
cp %{_builddir}/%{name}-%{version}/boops.service /etc/systemd/system/

# Reload systemd and enable the boops service
systemctl daemon-reload
systemctl enable boops.service
systemctl start boops.service

%files
/usr/local/bin/boops
/etc/boops/config.json
%{_sysconfdir}/systemd/system/boops.service

%changelog
