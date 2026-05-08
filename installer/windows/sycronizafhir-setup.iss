#define MyAppName "Agencia TA, Soluciones Empresariales"
#define MyAppVersion "1.4.0"
#define MyAppPublisher "Agencia TA, Soluciones Empresariales"
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
OutputBaseFilename=agencia-ta-soluciones-setup
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
Filename: "powershell.exe"; Parameters: "-NoProfile -ExecutionPolicy Bypass -File ""{tmp}\sycronizafhir-installer\instalar-sycronizafhir.ps1"""; StatusMsg: "Instalando WebView2 Runtime, servicios y configurando arranque en segundo plano..."; Flags: runhidden waituntilterminated
Filename: "powershell.exe"; Parameters: "-NoProfile -ExecutionPolicy Bypass -File ""{app}\abrir-monitor-sycronizafhir.ps1"""; Description: "Abrir aplicacion (monitor de sincronizacion)"; Flags: postinstall nowait skipifsilent unchecked

[UninstallRun]
Filename: "powershell.exe"; Parameters: "-NoProfile -ExecutionPolicy Bypass -File ""{app}\desinstalar-sycronizafhir.ps1"""; Flags: runhidden waituntilterminated

[Code]
procedure InitializeWizard;
begin
  WizardForm.WelcomeLabel1.Caption := 'Instalador de Agencia TA, Soluciones Empresariales';
  WizardForm.WelcomeLabel2.Caption :=
    'Este programa instala y configura la sincronizacion bidireccional entre base local y Supabase.'#13#10 +
    'Si tu equipo no tiene Microsoft Edge WebView2 Runtime, sera instalado automaticamente.'#13#10 +
    'La aplicacion quedara ejecutandose en segundo plano y podra abrirse desde el acceso directo del Escritorio.';
end;
