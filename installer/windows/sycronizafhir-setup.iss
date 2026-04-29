#define MyAppName "sycronizafhir"
#define MyAppVersion "1.0.0"
#define MyAppPublisher "Sycroniza"
#define MyAppExeName "sycronizafhir-setup-launcher.exe"

[Setup]
AppId={{7D75EAB2-A517-4B4C-B9A9-0E516603E98C}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
DefaultDirName={autopf}\sycronizafhir
DisableDirPage=yes
DisableProgramGroupPage=yes
OutputDir={#OutputDir}
OutputBaseFilename=sycronizafhir-setup
Compression=lzma
SolidCompression=yes
WizardStyle=modern
PrivilegesRequired=admin
ArchitecturesInstallIn64BitMode=x64compatible
UninstallDisplayIcon={app}\sycronizafhir.exe

[Languages]
Name: "spanish"; MessagesFile: "compiler:Languages\Spanish.isl"

[Files]
Source: "{#SourceDir}\*"; DestDir: "{tmp}\sycronizafhir-installer"; Flags: recursesubdirs createallsubdirs ignoreversion

[Run]
Filename: "powershell.exe"; Parameters: "-NoProfile -ExecutionPolicy Bypass -File ""{tmp}\sycronizafhir-installer\instalar-sycronizafhir.ps1"""; StatusMsg: "Instalando servicios y configurando arranque en segundo plano..."; Flags: runhidden waituntilterminated

[UninstallRun]
Filename: "powershell.exe"; Parameters: "-NoProfile -ExecutionPolicy Bypass -File ""{app}\desinstalar-sycronizafhir.ps1"""; Flags: runhidden waituntilterminated

[Code]
procedure InitializeWizard;
begin
  WizardForm.WelcomeLabel1.Caption := 'Instalador de sycronizafhir';
  WizardForm.WelcomeLabel2.Caption :=
    'Este programa instala y configura la sincronización bidireccional entre base local y Supabase.'#13#10 +
    'La aplicación quedará ejecutándose en segundo plano y podrá abrirse desde el acceso directo del escritorio.';
end;
