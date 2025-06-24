cd build-logic

candle LicenseAgreementDlg_HK.wxs WixUI_HK.wxs product.wxs

light -ext WixUIExtension -ext WixUtilExtension -cultures:ja-JP -sacl -spdb  -out ..\dist\SabaLauncher.msi LicenseAgreementDlg_HK.wixobj WixUI_HK.wixobj product.wixobj
